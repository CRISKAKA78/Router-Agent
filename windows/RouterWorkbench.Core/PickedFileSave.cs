using System.Security.Cryptography;
namespace RouterWorkbench.Core;
// One save capability, created only after a Windows Save Picker chose its target.
// The bridge sees an opaque handle, never a filesystem path.
public sealed class PickedFileSave : IAsyncDisposable
{
 public string Id { get; } = Guid.NewGuid().ToString("N");
 private readonly string target, temporary, expectedHash;
 private readonly long expectedSize;
 private readonly FileStream stream;
 private readonly IncrementalHash hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
 private long written;
 private bool closed;
 public PickedFileSave(string path, long size, string sha256) {
  if(size is < 0 or > 1073741824 || sha256.Length!=64 || sha256.Any(c=>!char.IsAsciiHexDigit(c)))throw new ArgumentException("无效文件大小或摘要。");
  target=Path.GetFullPath(path);temporary=target+"."+Id+".part";expectedHash=sha256;expectedSize=size;
  stream=new FileStream(temporary,FileMode.CreateNew,FileAccess.Write,FileShare.None,65536,true);
 }
 public async Task WriteAsync(long offset,string base64,CancellationToken ct) {
  ObjectDisposedException.ThrowIf(closed,this);
  if(offset!=written||base64.Length>87384)throw new ArgumentException("无效保存分块。");
  var bytes=Convert.FromBase64String(base64);
  if(bytes.Length is < 1 or > 65536 || written+bytes.Length>expectedSize)throw new ArgumentException("保存分块超出范围。");
  await stream.WriteAsync(bytes,ct);hash.AppendData(bytes);written+=bytes.Length;
 }
 public async Task CompleteAsync(CancellationToken ct) {
  ObjectDisposedException.ThrowIf(closed,this);
  if(written!=expectedSize||!Convert.ToHexStringLower(hash.GetHashAndReset()).Equals(expectedHash,StringComparison.OrdinalIgnoreCase))throw new InvalidDataException("保存文件完整性校验失败。");
  await stream.FlushAsync(ct);await stream.DisposeAsync();closed=true;File.Move(temporary,target,true);
 }
 public async ValueTask DisposeAsync(){if(!closed){await stream.DisposeAsync();closed=true;}hash.Dispose();if(File.Exists(temporary))File.Delete(temporary);}
}
