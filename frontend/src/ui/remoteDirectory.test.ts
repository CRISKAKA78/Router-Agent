import {describe,it,expect} from 'vitest';
import {directoryCommand,parseDirectory,shellQuote} from './remoteDirectory';
describe('remote directory task encoding',()=>{
  it('quotes user paths as a single literal shell argument',()=>{
    expect(shellQuote("/tmp/a'$(touch /tmp/unwanted)" )).toBe("'/tmp/a'\\''$(touch /tmp/unwanted)'");
    expect(directoryCommand('/tmp/a\nb')).toContain("cd '/tmp/a\nb' || exit 1");
    expect(()=>directoryCommand('relative')).toThrow();
    expect(()=>directoryCommand('/tmp/\x00bad')).toThrow();
  });
  it('preserves whitespace and newlines in filenames',()=>{
    const result=parseDirectory(['f','123','1700000000','a\n b\t.txt','d','4096','1700000001','文件夹',''].join('\x00'));
    expect(result.entries.map(e=>[e.name,e.folder,e.size])).toEqual([['a\n b\t.txt',false,123],['文件夹',true,4096]]);
    expect(result.limited).toBe(false);
  });
  it('reports bounded listings without pretending the directory is complete',()=>{
    expect(parseDirectory('LIMIT\x00')).toEqual({entries:[],limited:true});
    expect(parseDirectory('')).toEqual({entries:[],limited:false});
  });
  it('rejects truncated and malformed directory output',()=>{
    for(const output of ['f\x00','f\x00abc\x000\x00name\x00','f\x001\x000\x00../escape\x00','f\x001\x000\x00name'])expect(()=>parseDirectory(output)).toThrow();
  });
});
