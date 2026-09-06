param([Parameter(Mandatory)][int]$ProcessId,[Parameter(Mandatory)][string]$TargetPath,[string]$Title='另存文件')
$ErrorActionPreference='Stop'
Add-Type @'
using System; using System.Text; using System.Runtime.InteropServices;
public static class NativePicker {
 public delegate bool EnumProc(IntPtr h,IntPtr p);
 [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc f,IntPtr p);
 [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr h,EnumProc f,IntPtr p);
 [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h,int id);
 [DllImport("user32.dll")] public static extern int GetDlgCtrlID(IntPtr h);
 [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
 [DllImport("user32.dll")] public static extern bool IsWindowEnabled(IntPtr h);
 [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern IntPtr SendMessage(IntPtr h,uint m,IntPtr w,string s);
 [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
 public static bool Choose(uint process,string title,string path) {
  IntPtr dialog=IntPtr.Zero; EnumWindows((h,p)=>{uint pid;GetWindowThreadProcessId(h,out pid);var text=new StringBuilder(256);GetWindowText(h,text,256);if(pid==process&&text.ToString()==title&&IsWindowVisible(h)&&IsWindowEnabled(GetDlgItem(h,1)))dialog=h;return true;},IntPtr.Zero);
  if(dialog==IntPtr.Zero)return false;var filename=GetDlgItem(dialog,1148);
  if(filename==IntPtr.Zero)EnumChildWindows(dialog,(h,p)=>{var name=new StringBuilder(128);GetClassName(h,name,128);if(name.ToString()=="Edit"&&GetDlgCtrlID(h)==1001)filename=h;return true;},IntPtr.Zero);
  if(filename==IntPtr.Zero)return false;EnumChildWindows(filename,(h,p)=>{var name=new StringBuilder(128);GetClassName(h,name,128);if(name.ToString()=="Edit")filename=h;return true;},IntPtr.Zero);
  System.Threading.Thread.Sleep(300);SendMessage(filename,12,IntPtr.Zero,path);System.Threading.Thread.Sleep(100);SetForegroundWindow(dialog);PostMessage(GetDlgItem(dialog,1),245,IntPtr.Zero,IntPtr.Zero);return true;
 }
}
'@
for($attempt=0;$attempt -lt 100;$attempt++){if([NativePicker]::Choose($ProcessId,$Title,[IO.Path]::GetFullPath($TargetPath))){'PASS native file picker';exit 0};Start-Sleep -Milliseconds 100}
throw 'Native file picker did not become ready'
