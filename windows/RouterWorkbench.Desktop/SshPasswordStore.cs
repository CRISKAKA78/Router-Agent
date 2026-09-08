using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

namespace RouterWorkbench.Desktop;

// Only the current Windows user can decrypt this local preference. It never enters profile JSON or an API request.
public static class SshPasswordStore
{
    [StructLayout(LayoutKind.Sequential)] private struct Blob { public int Size; public IntPtr Data; }
    [DllImport("crypt32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    private static extern bool CryptProtectData(ref Blob input, string? description, IntPtr entropy, IntPtr reserved, IntPtr prompt, uint flags, out Blob output);
    [DllImport("crypt32.dll", SetLastError = true)]
    private static extern bool CryptUnprotectData(ref Blob input, IntPtr description, IntPtr entropy, IntPtr reserved, IntPtr prompt, uint flags, out Blob output);
    [DllImport("kernel32.dll")] private static extern IntPtr LocalFree(IntPtr memory);
    public static string Load(string profilePath)
    {
        if (!File.Exists(profilePath + ".ssh-password")) return "admin";
        var bytes = Transform(File.ReadAllBytes(profilePath + ".ssh-password"), false);
        try { return Encoding.UTF8.GetString(bytes); } finally { CryptographicOperations.ZeroMemory(bytes); }
    }
    public static async Task SaveAsync(string profilePath, string password)
    {
        if (password.Length > 1024 || password.Contains('\0') || password.Contains('\r') || password.Contains('\n')) throw new ArgumentException("SSH 密码不能超过 1024 字符或包含换行 / NUL。");
        var bytes = Encoding.UTF8.GetBytes(password); byte[] protectedBytes;
        try { protectedBytes = Transform(bytes, true); } finally { CryptographicOperations.ZeroMemory(bytes); }
        var path = profilePath + ".ssh-password"; var temp = path + ".tmp";
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        try { await File.WriteAllBytesAsync(temp, protectedBytes); File.Move(temp, path, true); }
        finally { if (File.Exists(temp)) File.Delete(temp); }
    }
    private static byte[] Transform(byte[] bytes, bool protect)
    {
        var input = new Blob { Size = bytes.Length, Data = Marshal.AllocHGlobal(Math.Max(1, bytes.Length)) }; Blob output = default;
        try {
            Marshal.Copy(bytes, 0, input.Data, bytes.Length);
            var ok = protect ? CryptProtectData(ref input, "Router Workbench SSH", IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, 1, out output)
                : CryptUnprotectData(ref input, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, 1, out output);
            if (!ok) throw new Win32Exception(Marshal.GetLastWin32Error(), "本机 SSH 密码加密或解密失败，请在设置重新输入。");
            var result = new byte[output.Size]; Marshal.Copy(output.Data, result, 0, output.Size); return result;
        } finally {
            for (var i = 0; i < input.Size; i++) Marshal.WriteByte(input.Data, i, 0);
            Marshal.FreeHGlobal(input.Data);
            if (output.Data != IntPtr.Zero) { for (var i = 0; i < output.Size; i++) Marshal.WriteByte(output.Data, i, 0); LocalFree(output.Data); }
        }
    }
}
