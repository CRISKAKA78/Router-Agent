param([Parameter(Mandatory)][string]$Executable)
$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using System.Text;
public static class WorkbenchPublishedWindow {
    public delegate bool EnumProc(IntPtr window, IntPtr parameter);
    [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc callback, IntPtr parameter);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr window, StringBuilder text, int length);
    [DllImport("user32.dll", EntryPoint="PostMessageW")] public static extern bool PostMessage(IntPtr window, uint message, UIntPtr wparam, IntPtr lparam);
    public static IntPtr Find(uint pid) {
        IntPtr found = IntPtr.Zero;
        EnumWindows((window, _) => {
            uint owner; GetWindowThreadProcessId(window, out owner);
            var text = new StringBuilder(256); GetWindowText(window, text, text.Capacity);
            if (owner == pid && text.ToString() == "远程维护工作台") found = window;
            return true;
        }, IntPtr.Zero);
        return found;
    }
}
'@
$releaseDirectory = Split-Path (Resolve-Path -LiteralPath $Executable).Path -Parent
$originalPath = $env:PATH
try {
    # Only this launch inherits the restricted PATH; the user's environment is restored.
    $env:PATH = "$env:SystemRoot\System32;$env:SystemRoot"
    if (Get-Command node.exe,dotnet.exe -ErrorAction SilentlyContinue) { throw 'Developer runtime remains on test PATH' }
    $publishedProcess = Start-Process -FilePath (Join-Path $releaseDirectory 'RouterWorkbench.exe') -WorkingDirectory $releaseDirectory -WindowStyle Hidden -PassThru
} finally { $env:PATH = $originalPath }
try {
    $window = [IntPtr]::Zero
    for ($attempt = 0; $attempt -lt 150 -and $window -eq [IntPtr]::Zero; $attempt++) {
        if ($publishedProcess.HasExited) { throw "发布程序启动失败，退出码 $($publishedProcess.ExitCode)" }
        Start-Sleep -Milliseconds 100
        $window = [WorkbenchPublishedWindow]::Find($publishedProcess.Id)
    }
    if ($window -eq [IntPtr]::Zero) { throw '发布程序未创建工作台窗口，请检查本地 last-error.log' }
    $web = $null
    for ($attempt = 0; $attempt -lt 120; $attempt++) {
        if ($publishedProcess.HasExited) { throw 'Product exited before WebView2 loaded' }
        $web = Get-CimInstance Win32_Process -Filter "Name='msedgewebview2.exe'" | Where-Object {
            $_.ParentProcessId -eq $publishedProcess.Id -and $_.ExecutablePath -eq (Join-Path $releaseDirectory 'WebView2Runtime/msedgewebview2.exe')
        }
        if ($web) { break }
        Start-Sleep -Milliseconds 500
    }
    if (!$web) { throw 'Product did not load its bundled WebView2 runtime' }
    $publishedProcess.Refresh()
    $xaml = $publishedProcess.Modules | Where-Object ModuleName -eq 'Microsoft.UI.Xaml.dll'
    if (!$xaml -or (Split-Path $xaml.FileName -Parent) -ne (Split-Path (Resolve-Path -LiteralPath $Executable).Path -Parent)) { throw '未从发布目录加载 WinUI 运行库' }
    [WorkbenchPublishedWindow]::PostMessage($window, 16, [UIntPtr]::Zero, [IntPtr]::Zero) | Out-Null
    if (!$publishedProcess.WaitForExit(15000) -or $publishedProcess.ExitCode -ne 0) { throw '发布程序未正常退出' }
    'PASS published native window, bundled WebView2/WinUI, no Node/dotnet on child PATH, WM_CLOSE exit=0'
}
finally { if (!$publishedProcess.HasExited) { $publishedProcess.Kill() }; $publishedProcess.Dispose() }
