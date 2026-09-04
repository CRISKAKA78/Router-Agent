#include "rmp/task_manager.h"
#include "rmp/json.h"
#include <chrono>
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <unistd.h>

namespace {
void Check(bool ok, const char* message) { if (!ok) throw std::runtime_error(message); }
template<class Predicate> void Wait(Predicate predicate) {
    for (int i = 0; i < 500; ++i) {
        if (predicate()) return;
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    throw std::runtime_error("task manager wait timed out");
}
}

int main() {
    try {
        char directory[] = "/tmp/rmp-manager-XXXXXX";
        Check(mkdtemp(directory) != NULL, "mkdtemp");
        const std::string base(directory), gate = base + "/gate", count = base + "/count";
        {
            rmp::TaskManager manager(1, 2, 65536);
            rmp::ExecTask first;
            first.task_id = "running"; first.type = "exec"; first.timeout = 5;
            first.command = "echo once >> " + count + "; while [ ! -f " + gate + " ]; do sleep 0.02; done; printf first";
            Check(manager.Submit(first, true, 500, 4096) == "queued", "first admission");
            Wait([&] { return manager.RunningTasks() == 1; });
            std::vector<std::thread> submitters;
            std::atomic<bool> duplicates_ok(true);
            for (int i = 0; i < 20; ++i) submitters.push_back(std::thread([&] {
                if (manager.Submit(first, true, 500, 4096) != "running") duplicates_ok.store(false);
            }));
            for (std::size_t i = 0; i < submitters.size(); ++i) submitters[i].join();
            Check(duplicates_ok.load(), "simultaneous duplicates");
            rmp::ExecTask queued = first; queued.task_id = "queued"; queued.command = "printf second";
            Check(manager.Submit(queued, true, 500, 4096) == "queued", "queued admission");
            Check(manager.Submit(queued, true, 500, 4096) == "queued", "queued duplicate");
            rmp::ExecTask conflict = first; conflict.command = "printf wrong";
            Check(manager.Submit(conflict, true, 500, 4096) == "conflict", "content conflict");
            Check(manager.Submit(first, false, 500, 4096) == "conflict", "invalid duplicate preserves task");
            rmp::ExecTask third = queued; third.task_id = "third";
            Check(manager.Submit(third, true, 500, 4096) == "rejected", "capacity rejection");
            manager.BeginSession(); // Disconnect/reconnect while first runs and second queues.
            std::ofstream(gate.c_str()) << "go";
            std::string original;
            Wait([&] { return manager.CachedResult(first.task_id, 4096, &original); });
            Wait([&] { return manager.Submit(queued, true, 500, 4096) == "success"; });
            Check(manager.Submit(first, true, 500, 4096) == "success", "completed duplicate");
            Check(manager.Submit(third, true, 500, 4096) == "rejected", "completed identity was evicted");
            for (int reconnect = 0; reconnect < 3; ++reconnect) {
                manager.BeginSession();
                std::string payload;
                Check(manager.NextResult(4096, &payload), "replay one");
                Check(manager.NextResult(4096, &payload), "replay two");
                Check(!manager.NextResult(4096, &payload), "duplicate spontaneous replay in session");
                Check(manager.CachedResult(first.task_id, 4096, &payload) && payload == original, "immutable cached result");
            }
            std::ifstream input(count.c_str()); std::string line; int lines = 0;
            while (std::getline(input, line)) ++lines;
            Check(lines == 1, "side effect executed more than once");
        }
        unlink(gate.c_str()); unlink(count.c_str()); rmdir(base.c_str());
        {
            rmp::TaskManager manager(1, 10, 6000);
            rmp::ExecTask task; task.task_id = "large"; task.type = "exec"; task.timeout = 2;
            task.command = "head -c 2000 /dev/zero";
            Check(manager.Submit(task, true, 200, 4096) == "queued", "byte admission");
            rmp::ExecTask other = task; other.task_id = "other";
            Check(manager.Submit(other, true, 200, 4096) == "rejected", "byte reservation not enforced");
            std::string payload;
            Wait([&] { return manager.CachedResult(task.task_id, 4096, &payload); });
            Check(payload.size() > 1024, "large cached payload");
            manager.BeginSession();
            Check(!manager.NextResult(1024, &payload), "changed immutable result for smaller session");
            Check(manager.NextResult(4096, &payload), "larger session cannot replay retained result");
        }
        std::cout << "task manager tests passed\n";
        return 0;
    } catch (const std::exception& error) { std::cerr << error.what() << '\n'; return 1; }
}
