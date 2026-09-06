using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Channels;
using Microsoft.Win32.SafeHandles;

namespace RouterWorkbench.Core;

// Windows-only platform adapter. No SSH/Telnet protocol, API or device state here.
public sealed class EmbeddedTerminal : IAsyncDisposable
{
    public string Id { get; } = Guid.NewGuid().ToString("N");
    private readonly Channel<byte[]> output = Channel.CreateBounded<byte[]>(32);
    private readonly Channel<byte[]> input = Channel.CreateBounded<byte[]>(16);
    private readonly CancellationTokenSource stopping = new();
    private readonly FileStream reader, writer;
    private readonly Process process;
    private readonly Task readTask, writeTask, exitTask;
    private readonly object consoleGate = new();
    private nint console;
    private Task? disposal;
    public bool Exited => process.HasExited && output.Reader.Completion.IsCompleted;

    public static ProcessStartInfo Client(Endpoint endpoint, ServerProfile profile)
    {
        ShellPolicy.ValidateEndpoint(endpoint);
        if(endpoint.Service is not ("ssh" or "telnet"))throw new ArgumentException("仅支持 SSH/Telnet 终端。");
        var system = Environment.GetFolderPath(Environment.SpecialFolder.System);
        var selected = endpoint.Service == "ssh" ? profile.SshExecutable : profile.TelnetExecutable;
        // GUI PuTTY remains an external option; never embed its window.
        if(string.IsNullOrWhiteSpace(selected) || Path.GetFileName(selected).Equals("putty.exe", StringComparison.OrdinalIgnoreCase))
            selected=Path.Combine(system, endpoint.Service=="ssh"?@"OpenSSH\ssh.exe":"telnet.exe");
        var name=Path.GetFileName(selected).ToLowerInvariant();
        if(!Path.IsPathFullyQualified(selected)||!File.Exists(selected)||name != endpoint.Service+".exe")
            throw new FileNotFoundException($"未找到内置终端所需的 {endpoint.Service}.exe。请安装系统客户端，或从外部工具入口连接。",selected);
        var info=new ProcessStartInfo(selected){UseShellExecute=false,CreateNoWindow=true};
        if(endpoint.Service=="ssh"){
            ShellPolicy.ValidateProfile(profile,profile);
            foreach(var arg in new[]{"-F","NUL","-o","ClearAllForwardings=yes","-o","PermitLocalCommand=no","-p",endpoint.Port.ToString(System.Globalization.CultureInfo.InvariantCulture),"-l",profile.SshUser,endpoint.Host})info.ArgumentList.Add(arg);
        } else {info.ArgumentList.Add(endpoint.Host);info.ArgumentList.Add(endpoint.Port.ToString(System.Globalization.CultureInfo.InvariantCulture));}
        return info;
    }

    public EmbeddedTerminal(ProcessStartInfo client, int columns, int rows)
    {
        ValidateSize(columns,rows);
        if(!OperatingSystem.IsWindows())throw new PlatformNotSupportedException("内置终端需要 Windows。");
        SafeFileHandle? inputRead=null,inputWrite=null,outputRead=null,outputWrite=null;
        nint attributes=0; Process? child=null;
        try {
            if(!CreatePipe(out inputRead,out inputWrite,0,0)||!CreatePipe(out outputRead,out outputWrite,0,0))throw new Win32Exception();
            Marshal.ThrowExceptionForHR(CreatePseudoConsole(new((short)columns,(short)rows),inputRead,outputWrite,0,out console));
            inputRead.Dispose();inputRead=null;outputWrite.Dispose();outputWrite=null;
            nuint bytes=0;InitializeProcThreadAttributeList(0,1,0,ref bytes);
            attributes=Marshal.AllocHGlobal(checked((int)bytes));
            if(!InitializeProcThreadAttributeList(attributes,1,0,ref bytes))throw new Win32Exception();
            if(!UpdateProcThreadAttribute(attributes,0,(nuint)0x00020016,console,(nuint)nint.Size,0,0))throw new Win32Exception();
            var startup=new StartupInfoEx{StartupInfo=new StartupInfo{Cb=Marshal.SizeOf<StartupInfoEx>(),Flags=0x100},AttributeList=attributes};
            var command=new StringBuilder(string.Join(" ",new[]{client.FileName}.Concat(client.ArgumentList).Select(Quote)));
            if(!CreateProcess(client.FileName,command,0,0,false,0x00080000,0,null,ref startup,out var pi))throw new Win32Exception();
            try{child=Process.GetProcessById(pi.ProcessId);}finally{CloseHandle(pi.Process);CloseHandle(pi.Thread);}
            reader=new FileStream(outputRead,FileAccess.Read,4096,false);outputRead=null;
            writer=new FileStream(inputWrite,FileAccess.Write,4096,false);inputWrite=null;
            process=child;child=null;
            readTask=Task.Run(ReadLoop);
            writeTask=Task.Run(WriteLoop);
            exitTask=Task.Run(async()=>{
                await process.WaitForExitAsync();
                // Closing ConPTY after natural client exit flushes the final bytes and signals EOF.
                nint handle;lock(consoleGate){handle=console;console=0;}
                if(handle!=0)ClosePseudoConsole(handle);
            });
        } catch {
            if(child!=null){try{child.Kill(true);}catch(InvalidOperationException){}child.Dispose();}
            // Closing the read end before ConPTY teardown prevents undrained output on construction failure.
            outputRead?.Dispose();outputRead=null;
            if(console!=0){ClosePseudoConsole(console);console=0;}
            throw;
        } finally {
            if(attributes!=0){DeleteProcThreadAttributeList(attributes);Marshal.FreeHGlobal(attributes);}
            inputRead?.Dispose();inputWrite?.Dispose();outputRead?.Dispose();outputWrite?.Dispose();
        }
    }
    private async Task ReadLoop(){
        try {
            var buffer=new byte[4096];
            for(int n;(n=reader.Read(buffer,0,buffer.Length))>0;){
                if(stopping.IsCancellationRequested)continue;
                try{await output.Writer.WriteAsync(buffer.AsSpan(0,n).ToArray(),stopping.Token);}catch(OperationCanceledException){}
            }
        } catch(IOException){} finally{output.Writer.TryComplete();}
    }
    private async Task WriteLoop(){
        try{await foreach(var bytes in input.Reader.ReadAllAsync(stopping.Token)){writer.Write(bytes);writer.Flush();}}
        catch(OperationCanceledException){}catch(IOException){}
    }
    public object Read(){
        using var buffer=new MemoryStream();
        while(buffer.Length<32768&&output.Reader.TryRead(out var bytes))buffer.Write(bytes);
        return new {base64=Convert.ToBase64String(buffer.ToArray()),exited=Exited,exit_code=Exited?process.ExitCode:(int?)null};
    }
    public void Write(string text){
        if(stopping.IsCancellationRequested||process.HasExited)throw new InvalidOperationException("终端已关闭。");
        var bytes=Encoding.UTF8.GetBytes(text);
        if(bytes.Length>16384)throw new ArgumentException("单次终端输入过大。");
        if(!input.Writer.TryWrite(bytes))throw new InvalidOperationException("终端输入队列已满，请稍后重试。");
    }
    public void Resize(int columns,int rows){ValidateSize(columns,rows);lock(consoleGate){if(console!=0)Marshal.ThrowExceptionForHR(ResizePseudoConsole(console,new((short)columns,(short)rows)));}}
    public static void ValidateSize(int columns,int rows){if(columns is < 2 or > 500 || rows is < 2 or > 300)throw new ArgumentException("无效终端尺寸。");}
    // Windows command-line quoting, including backslashes immediately before quotes/end.
    public static string Quote(string value){var result=new StringBuilder("\"");var slashes=0;foreach(var c in value){if(c=='\\'){slashes++;continue;}result.Append('\\',c=='"'?slashes*2+1:slashes);result.Append(c);slashes=0;}return result.Append('\\',slashes*2).Append('"').ToString();}
    public ValueTask DisposeAsync(){lock(this){disposal??=CloseAsync();return new(disposal);}}
    private async Task CloseAsync(){
        stopping.Cancel();input.Writer.TryComplete();
        try{if(!process.HasExited)process.Kill(true);}catch(InvalidOperationException){}
        // Keep the output thread draining while ClosePseudoConsole flushes final bytes.
        await exitTask;
        writer.Dispose();await Task.WhenAll(readTask,writeTask);reader.Dispose();
        await process.WaitForExitAsync();process.Dispose();stopping.Dispose();
    }

    [StructLayout(LayoutKind.Sequential)] private readonly record struct Coord(short X,short Y);
    [StructLayout(LayoutKind.Sequential,CharSet=CharSet.Unicode)] private struct StartupInfo {public int Cb;public nint Reserved,Desktop,Title;public int X,Y,XSize,YSize,XCountChars,YCountChars,FillAttribute,Flags;public short ShowWindow,Reserved2;public nint ReservedPointer,StdInput,StdOutput,StdError;}
    [StructLayout(LayoutKind.Sequential)] private struct StartupInfoEx {public StartupInfo StartupInfo;public nint AttributeList;}
    [StructLayout(LayoutKind.Sequential)] private struct ProcessInfo {public nint Process,Thread;public int ProcessId,ThreadId;}
    [DllImport("kernel32.dll",SetLastError=true)] [return:MarshalAs(UnmanagedType.Bool)] private static extern bool CreatePipe(out SafeFileHandle read,out SafeFileHandle write,nint attributes,uint size);
    [DllImport("kernel32.dll")] private static extern int CreatePseudoConsole(Coord size,SafeFileHandle input,SafeFileHandle output,uint flags,out nint console);
    [DllImport("kernel32.dll")] private static extern int ResizePseudoConsole(nint console,Coord size);
    [DllImport("kernel32.dll")] private static extern void ClosePseudoConsole(nint console);
    [DllImport("kernel32.dll",SetLastError=true)] [return:MarshalAs(UnmanagedType.Bool)] private static extern bool InitializeProcThreadAttributeList(nint list,int count,int flags,ref nuint size);
    [DllImport("kernel32.dll",SetLastError=true)] [return:MarshalAs(UnmanagedType.Bool)] private static extern bool UpdateProcThreadAttribute(nint list,uint flags,nuint attribute,nint value,nuint size,nint previous,nint returned);
    [DllImport("kernel32.dll")] private static extern void DeleteProcThreadAttributeList(nint list);
    [DllImport("kernel32.dll",EntryPoint="CreateProcessW",CharSet=CharSet.Unicode,SetLastError=true)] [return:MarshalAs(UnmanagedType.Bool)] private static extern bool CreateProcess(string application,StringBuilder command,nint processAttributes,nint threadAttributes,bool inheritHandles,uint flags,nint environment,string? directory,ref StartupInfoEx startup,out ProcessInfo information);
    [DllImport("kernel32.dll")] [return:MarshalAs(UnmanagedType.Bool)] private static extern bool CloseHandle(nint handle);
}
