using System;
using System.IO;

internal static class SshAskPass
{
    private static int Main()
    {
        try
        {
            Console.Write(File.ReadAllText(Environment.GetEnvironmentVariable("RMP_KEY_PASSWORD_FILE")).TrimEnd('\r', '\n'));
            return 0;
        }
        catch { return 1; }
    }
}
