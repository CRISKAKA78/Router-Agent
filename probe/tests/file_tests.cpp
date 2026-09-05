#include "rmp/file_wire.h"
#include "rmp/priority_gate.h"
#include "rmp/sha256.h"
#include "rmp/task_manager.h"
#include <atomic>
#include <chrono>
#include <iostream>
#include <stdexcept>
namespace {
void Check(bool b, const char *s) {
    if (!b)
        throw std::runtime_error(s);
}
} // namespace
int main() {
    try {
        struct Vector {
            std::string data, digest;
        };
        Vector vectors[] = {
            {"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
            {"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
            {std::string(1000000, 'a'), "cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0"}};
        for (unsigned i = 0; i < 3; i++) {
            rmp::Sha256 h;
            for (std::size_t j = 0; j < vectors[i].data.size(); j++)
                h.Update(&vectors[i].data[j], 1);
            Check(h.Finish() == vectors[i].digest, "SHA-256 vector");
        }
        const std::string id = "00112233-4455-4677-8899-aabbccddeeff";
        std::string uuid;
        Check(rmp::FileUUID(id, &uuid), "UUID");
        Check(static_cast<unsigned char>(uuid[4]) == 0x44 && static_cast<unsigned char>(uuid[6]) == 0x46,
              "UUID byte order");
        std::string error;
        bool accepted = false;
        for (unsigned i = 0; i < 3; i++) {
            std::string status = i == 0 ? "ready" : i == 1 ? "done" : "failed";
            std::string raw = rmp::FileAckPayload(9, id, status, 0);
            Check(rmp::CheckFileAck(raw, 9, id, i == 2 ? "done" : status, 0, &accepted, &error),
                  "ACK parses");
            if (i == 0) {
                Check(raw.find("sha256_ok") == std::string::npos, "ready omission");
                raw.pop_back();
                raw += ",\"sha256_ok\":false}";
                Check(!rmp::CheckFileAck(raw, 9, id, "ready", 0, &accepted, &error), "ready rejects false");
            }
        }
        rmp::FileParams p;
        p.transfer_id = id;
        p.size = 3;
        std::string chunk = rmp::FileChunkPayload(id, 0, "abc", 3);
        std::vector<std::uint8_t> bytes(chunk.begin(), chunk.end());
        Check(rmp::CheckFileChunk(bytes, p, 0, 3, &error), "chunk");
        Check(!rmp::CheckFileChunk(bytes, p, 1, 3, &error), "offset rejected");
        Check(!rmp::CheckFileChunk(bytes, p, 0, 2, &error), "chunk limit");
        rmp::TaskManager manager(1, 32, 65536, 1);
        rmp::ExecTask first;
        first.task_id = "first";
        first.type = "upload";
        first.timeout = 3;
        first.file = p;
        bool fresh = false;
        Check(manager.Submit(first, true, 200, 1024, &fresh) == "queued" && fresh, "file admitted");
        manager.FileRunning(first.task_id);
        rmp::ExecTask second = first;
        second.task_id = "second";
        second.file.transfer_id = "10112233-4455-4677-8899-aabbccddeeff";
        Check(manager.Submit(second, true, 200, 1024) == "queued", "file queue");
        Check(manager.Submit(second, true, 200, 1024, &fresh) == "queued" && !fresh, "duplicate queue");
        rmp::ExecTask third = second;
        third.task_id = "third";
        third.file.transfer_id = "20112233-4455-4677-8899-aabbccddeeff";
        Check(manager.Submit(third, true, 200, 1024) == "rejected", "FIFO capacity");
        rmp::ExecTask conflict = first;
        conflict.file.overwrite = true;
        Check(manager.Submit(conflict, true, 200, 1024) == "conflict", "file identity conflict");
        manager.FileComplete(first.task_id, "success", "{}");
        Check(manager.Submit(first, true, 200, 1024) == "success", "terminal idempotency");
        third.file.transfer_id = id;
        Check(manager.Submit(third, true, 200, 1024) == "rejected", "transfer reuse");
        manager.FileComplete(second.task_id, "failed", "{}");
        manager.BeginSession();
        std::string result;
        Check(manager.NextResult(1024, &result) && manager.NextResult(1024, &result) &&
                  !manager.NextResult(1024, &result),
              "file result replay");
        std::cout << "file tests passed\n";
        return 0;
    } catch (const std::exception &e) {
        std::cerr << e.what() << '\n';
        return 1;
    }
}
