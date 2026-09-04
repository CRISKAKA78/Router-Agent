#include "rmp/client.h"
#include "rmp/frame.h"
#include "rmp/json.h"
#include "rmp/task.h"

#include <atomic>
#include <cerrno>
#include <chrono>
#include <cstdlib>
#include <csignal>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>
#include <unistd.h>
#include <vector>

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
    if (!condition) {
        std::cerr << "FAIL: " << message << std::endl;
        ++failures;
    }
}

std::vector<std::uint8_t> MakeFrame(std::uint64_t message_id, const std::string& payload) {
    rmp::Header header;
    header.type = rmp::kTypeHeartbeat;
    header.message_id = message_id;
    return rmp::EncodeFrame(header, std::vector<std::uint8_t>(payload.begin(), payload.end()));
}

void TestHeaderAndBigEndian() {
    rmp::Header header;
    header.type = rmp::kTypeHeartbeat;
    header.flags = 0x0102;
    header.payload_len = 0x03040506;
    header.message_id = UINT64_C(0x0708090A0B0C0D0E);
    std::uint8_t encoded[rmp::kHeaderSize];
    rmp::EncodeHeader(header, encoded);
    Check(std::string(reinterpret_cast<char*>(encoded), 4) == "RMP1", "header magic");
    Check(encoded[6] == 0x01 && encoded[7] == 0x02, "flags use Big Endian");
    Check(encoded[8] == 0x03 && encoded[9] == 0x04 && encoded[10] == 0x05 && encoded[11] == 0x06,
          "payload_len uses Big Endian");
    Check(encoded[12] == 0x07 && encoded[19] == 0x0E, "message_id uses Big Endian");

    rmp::Header decoded;
    rmp::FrameErrorCode error = rmp::FrameErrorCode::kNone;
    Check(rmp::DecodeHeader(encoded, sizeof(encoded), UINT32_MAX, &decoded, &error), "header decodes");
    Check(decoded.version == header.version && decoded.type == header.type &&
              decoded.flags == header.flags && decoded.payload_len == header.payload_len &&
              decoded.message_id == header.message_id,
          "header round trip");
}

void TestStreamFraming() {
    const std::vector<std::uint8_t> first = MakeFrame(1, "abc");
    rmp::StreamDecoder half_header;
    std::vector<rmp::Frame> frames;
    rmp::FrameErrorCode error = rmp::FrameErrorCode::kNone;
    Check(half_header.Feed(&first[0], 10, &frames, &error), "half header accepted");
    Check(frames.empty(), "half header emits no frame");
    Check(half_header.Feed(&first[10], first.size() - 10, &frames, &error), "second header part accepted");
    Check(frames.size() == 1 && frames[0].header.message_id == 1, "half header completes frame");

    rmp::StreamDecoder half_payload;
    frames.clear();
    Check(half_payload.Feed(&first[0], rmp::kHeaderSize + 1, &frames, &error), "half payload accepted");
    Check(frames.empty(), "half payload emits no frame");
    Check(half_payload.Feed(&first[rmp::kHeaderSize + 1], first.size() - rmp::kHeaderSize - 1,
                           &frames, &error),
          "second payload part accepted");
    Check(frames.size() == 1 && std::string(frames[0].payload.begin(), frames[0].payload.end()) == "abc",
          "half payload completes frame");

    const std::vector<std::uint8_t> second = MakeFrame(2, "def");
    std::vector<std::uint8_t> combined = first;
    combined.insert(combined.end(), second.begin(), second.end());
    rmp::StreamDecoder sticky;
    frames.clear();
    Check(sticky.Feed(&combined[0], combined.size(), &frames, &error), "combined frames accepted");
    Check(frames.size() == 2 && frames[0].header.message_id == 1 && frames[1].header.message_id == 2,
          "multiple frames decoded from one read");
}

void TestHeaderErrors() {
    std::vector<std::uint8_t> encoded = MakeFrame(1, "x");
    rmp::Header header;
    rmp::FrameErrorCode error = rmp::FrameErrorCode::kNone;

    encoded[0] = 'X';
    Check(!rmp::DecodeHeader(&encoded[0], rmp::kHeaderSize, rmp::kMaxControlPayload, &header, &error) &&
              error == rmp::FrameErrorCode::kBadMagic,
          "bad magic rejected");

    encoded = MakeFrame(1, "x");
    encoded[4] = 2;
    Check(!rmp::DecodeHeader(&encoded[0], rmp::kHeaderSize, rmp::kMaxControlPayload, &header, &error) &&
              error == rmp::FrameErrorCode::kUnsupportedVersion,
          "unsupported version rejected");

    encoded = MakeFrame(1, "x");
    encoded[8] = 0x00;
    encoded[9] = 0x10;
    encoded[10] = 0x00;
    encoded[11] = 0x01;
    Check(!rmp::DecodeHeader(&encoded[0], rmp::kHeaderSize, 1024, &header, &error) &&
              error == rmp::FrameErrorCode::kPayloadTooLarge,
          "payload limit enforced");
}

void TestResponseJson() {
    const std::string failure_json =
        "{\"reply_to\":1,\"success\":false,\"error_code\":\"INVALID_REGISTER\","
        "\"message\":\"device_id is required\",\"retry_after\":30}";
    rmp::RegisterAck register_ack;
    std::string error;
    Check(rmp::ParseRegisterAck(failure_json, &register_ack, &error),
          "REGISTER_ACK success=false parses: " + error);
    Check(!register_ack.success && register_ack.reply_to == 1 &&
              register_ack.error_code == "INVALID_REGISTER" && register_ack.retry_after == 30,
          "REGISTER_ACK failure fields");

    const std::string success_json =
        "{\"reply_to\":1,\"success\":true,\"session_id\":\"sess_test\","
        "\"heartbeat_interval\":30,\"server_time\":1,"
        "\"max_control_payload\":1048576,\"file_chunk_size\":65536,"
        "\"future\":{\"ignored\":true}}";
    error.clear();
    Check(rmp::ParseRegisterAck(success_json, &register_ack, &error),
          "REGISTER_ACK success parses: " + error);
    Check(register_ack.success && register_ack.session_id == "sess_test" &&
              register_ack.heartbeat_interval == 30,
          "REGISTER_ACK success fields");

    rmp::HeartbeatAck heartbeat_ack;
    error.clear();
    Check(rmp::ParseHeartbeatAck("{\"reply_to\":27,\"server_time\":2}", &heartbeat_ack, &error),
          "HEARTBEAT_ACK parses: " + error);
    Check(heartbeat_ack.reply_to == 27 && heartbeat_ack.server_time == 2,
          "HEARTBEAT_ACK reply_to");
}

void TestServerAddress() {
    std::string host;
    std::string port;
    std::string error;
    Check(rmp::ParseServerAddress("127.0.0.1:9000", &host, &port, &error) &&
              host == "127.0.0.1" && port == "9000",
          "IPv4 server address");
    Check(rmp::ParseServerAddress("[::1]:9000", &host, &port, &error) && host == "::1",
          "IPv6 server address");
}

void TestTaskParsingAndAck() {
    const std::string payload =
        "{\"task_id\":\"task-1\",\"type\":\"exec\",\"created_at\":1,\"timeout\":5,"
        "\"params\":{\"command\":\"printf \\\"$VALUE\\\"\",\"cwd\":\"/tmp\","
        "\"env\":{\"VALUE\":\"hello\"}},\"future\":true}";
    rmp::ExecTask task;
    std::string error;
    Check(rmp::ParseTask(payload, &task, &error), "exec TASK parses: " + error);
    Check(task.task_id == "task-1" && task.type == "exec" && task.timeout == 5 &&
              task.cwd == "/tmp" && task.env["VALUE"] == "hello",
          "exec TASK fields");

    error.clear();
    Check(rmp::ParseTask(
              "{\"task_id\":\"task-2\",\"type\":\"upload\",\"timeout\":5,\"params\":{}}",
              &task, &error) && task.type == "upload",
          "unsupported TASK remains parseable for rejection");

    rmp::JsonObject ack;
    error.clear();
    Check(rmp::ParseJsonObject(rmp::TaskAckPayload(7, "task-2", false, "unsupported task type"),
                               &ack, &error),
          "rejected TASK_ACK is JSON: " + error);
    Check(ack["reply_to"].unsigned_value == 7 && !ack["accepted"].bool_value &&
              ack["state"].string_value == "rejected",
          "rejected TASK_ACK fields");
}

void TestExecResultModes() {
    std::atomic<bool> stop(false);
    rmp::ExecTask task;
    task.task_id = "exec-success";
    task.type = "exec";
    task.timeout = 5;
    task.command = "printf out; printf err >&2; printf \"$VALUE\"";
    task.env["VALUE"] = "-env";
    rmp::ExecResult result = rmp::ExecuteExec(task, &stop);
    Check(result.status == "success" && result.exit_code == 0,
          "exec success result");
    Check(result.stdout_text == "out-env" && result.stderr_text == "err",
          "stdout and stderr captured separately");

    task.task_id = "exec-failed";
    task.command = "exit 7";
    result = rmp::ExecuteExec(task, &stop);
    Check(result.status == "failed" && result.exit_code == 7,
          "non-zero exec result");
}

void TestExecTimeoutAndReap() {
    std::ostringstream path_builder;
    path_builder << "/tmp/rmp-probe-timeout-" << static_cast<unsigned long>(getpid());
    const std::string pid_path = path_builder.str();
    unlink(pid_path.c_str());

    rmp::ExecTask task;
    task.task_id = "exec-timeout";
    task.type = "exec";
    task.timeout = 1;
    task.command = "echo $$ > " + pid_path + "; sleep 10";
    std::atomic<bool> stop(false);
    const std::chrono::steady_clock::time_point started = std::chrono::steady_clock::now();
    const rmp::ExecResult result = rmp::ExecuteExec(task, &stop);
    const std::chrono::seconds elapsed =
        std::chrono::duration_cast<std::chrono::seconds>(std::chrono::steady_clock::now() - started);
    Check(result.status == "timeout" && elapsed.count() < 4,
          "timeout terminates promptly");

    std::ifstream pid_file(pid_path.c_str());
    long child_pid = 0;
    pid_file >> child_pid;
    errno = 0;
    Check(child_pid > 0 && kill(static_cast<pid_t>(child_pid), 0) == -1 && errno == ESRCH,
          "timed out shell is killed and reaped");
    unlink(pid_path.c_str());
}

void TestExecOutputLimit() {
    rmp::ExecTask task;
    task.task_id = "exec-output-limit";
    task.type = "exec";
    task.timeout = 10;
    task.command = "head -c 1100000 /dev/zero >&2; printf done";
    std::atomic<bool> stop(false);
    rmp::ExecResult result = rmp::ExecuteExec(task, &stop);
    Check(result.status == "success" && result.stdout_text == "done",
          "large stderr does not block stdout");
    Check(result.stderr_text.size() == rmp::kMaxTaskOutput && result.truncated,
          "task output is capped and marked truncated");
    std::string payload;
    std::string error;
    Check(rmp::BuildTaskResultPayload(result, rmp::kMaxControlPayload, &payload, &error) &&
              payload.size() <= rmp::kMaxControlPayload,
          "TASK_RESULT fits negotiated control payload: " + error);
}

}  // namespace

int main() {
    TestHeaderAndBigEndian();
    TestStreamFraming();
    TestHeaderErrors();
    TestResponseJson();
    TestServerAddress();
    TestTaskParsingAndAck();
    TestExecResultModes();
    TestExecTimeoutAndReap();
    TestExecOutputLimit();
    if (failures != 0) {
        std::cerr << failures << " test assertion(s) failed" << std::endl;
        return EXIT_FAILURE;
    }
    std::cout << "probe_tests passed" << std::endl;
    return EXIT_SUCCESS;
}
