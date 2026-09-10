#include "rmp/task_manager.h"
#include "rmp/json.h"
#include "rmp/neighbors.h"
#include <ctime>
#include <iostream>
#include <stdexcept>

namespace rmp {
namespace {
bool SameTask(const ExecTask& a, const ExecTask& b) {
    return a.type == b.type && a.timeout == b.timeout && a.command == b.command &&
           a.cwd == b.cwd && a.env == b.env && a.file == b.file && a.config == b.config;
}
}

TaskManager::TaskManager(unsigned workers, std::size_t capacity, std::size_t byte_capacity, std::size_t file_capacity)
    : config_running_(false), stop_(false), running_(0), file_capacity_(file_capacity), file_count_(0), capacity_(capacity), byte_capacity_(byte_capacity),
      reserved_bytes_(0) {
    if (workers == 0 || workers > 64 || capacity == 0) {
        throw std::invalid_argument("invalid task manager limits");
    }
    try {
        for (unsigned i = 0; i < workers; ++i) workers_.push_back(std::thread(&TaskManager::Run, this));
    } catch (...) {
        stop_.store(true);
        condition_.notify_all();
        for (std::size_t i = 0; i < workers_.size(); ++i) workers_[i].join();
        throw;
    }
}

TaskManager::~TaskManager() {
    {std::lock_guard<std::mutex> lock(mutex_);for(auto& item:entries_)*item.second.cancel=true;}
    stop_.store(true);
    condition_.notify_all();
    for (std::size_t i = 0; i < workers_.size(); ++i) workers_[i].join();
}

std::string TaskManager::Submit(const ExecTask& task, bool valid,
                                std::size_t input_size, std::uint32_t max_payload, bool* fresh) {
    if (fresh) *fresh=false;
    std::lock_guard<std::mutex> lock(mutex_);
    std::map<std::string, Entry>::iterator existing = entries_.find(task.task_id);
    if (existing != entries_.end()) {
        if (!valid || !SameTask(existing->second.task, task)) return "conflict";
        return existing->second.state;
    }
    // Reserve the maximum encoded result at admission, including while offline.
    // Two input copies worth of room cover decoded strings and identity storage.
    const std::size_t reservation = input_size * 2 + max_payload;
    const bool file=task.type=="upload" || task.type=="download";
    if (file) {
        // Reserve enough for immutable file terminal metadata even for escaped IDs.
        if (EscapeJsonString(task.task_id).size()+420 > max_payload) return "rejected";
        if(file_count_>=file_capacity_+1) return "rejected";
        for(std::map<std::string,Entry>::const_iterator i=entries_.begin();i!=entries_.end();++i)
            if(i->second.task.file.transfer_id==task.file.transfer_id) return "rejected";
    }
    if (!valid || (task.type != "exec" && task.type != "router_config" && task.type != "neighbor_scan" && task.type != "neighbor_cancel" && !file) || entries_.size() >= capacity_ ||
        reservation > byte_capacity_ - reserved_bytes_) return "rejected";
    Entry entry;
    entry.task = task;
    entry.state = "queued";
    entry.max_payload = max_payload;
    entries_.insert(std::make_pair(task.task_id, entry));
    reserved_bytes_ += reservation;
    if (fresh) *fresh=true;
    if(task.type=="neighbor_cancel"){
        auto target=entries_.find(task.config.at("target_task_id"));ExecResult result;result.task_id=task.task_id;result.started_at=result.finished_at=static_cast<std::uint64_t>(std::time(NULL));
        bool found=target!=entries_.end()&&target->second.task.type=="neighbor_scan";
        if(found)*target->second.cancel=true;
        result.status=found?"success":"failed";result.exit_code=found?0:1;result.stdout_text=found?"stop_requested":"";result.stderr_text=found?"":"scan_not_found";
        auto& stored=entries_.at(task.task_id);std::string error;BuildTaskResultPayload(result,max_payload,&stored.payload,&error);stored.state=result.status;reserved_bytes_-=max_payload-stored.payload.size();return stored.state;
    }
    if (file) ++file_count_; else queue_.push_back(task.task_id);
    condition_.notify_one();
    return "queued";
}

void TaskManager::FileRunning(const std::string& id) {
    std::lock_guard<std::mutex> lock(mutex_);entries_.at(id).state="running";running_.fetch_add(1);
}
void TaskManager::FileComplete(const std::string& id,const std::string& status,const std::string& payload) {
    std::lock_guard<std::mutex> lock(mutex_);Entry& e=entries_.at(id);
    if(!e.payload.empty()) return;
    if(payload.size()>e.max_payload) throw std::length_error("file result exceeds reservation");
    if(e.state=="running") running_.fetch_sub(1);
    e.state=status;e.payload=payload;e.sent=true; // file worker sends in causal order; reconnect replays.
    reserved_bytes_-=e.max_payload-payload.size();--file_count_;
}

void TaskManager::BeginSession() {
    std::lock_guard<std::mutex> lock(mutex_);
    for (std::map<std::string, Entry>::iterator it = entries_.begin(); it != entries_.end(); ++it)
        it->second.sent = false;
}

bool TaskManager::NextResult(std::uint32_t max_payload, std::string* payload) {
    std::lock_guard<std::mutex> lock(mutex_);
    for (std::map<std::string, Entry>::iterator it = entries_.begin(); it != entries_.end(); ++it) {
        Entry& entry = it->second;
        if (!entry.sent && !entry.payload.empty() && entry.payload.size() <= max_payload) {
            *payload = entry.payload;
            entry.sent = true; // Any write failure ends this session; BeginSession replays it.
            return true;
        }
    }
    return false;
}

bool TaskManager::CachedResult(const std::string& id, std::uint32_t max_payload,
                               std::string* payload) {
    std::lock_guard<std::mutex> lock(mutex_);
    Entry& entry = entries_.at(id);
    if (entry.payload.empty() || entry.payload.size() > max_payload) return false;
    *payload = entry.payload;
    entry.sent = true;
    return true;
}

unsigned TaskManager::RunningTasks() const { return running_.load(); }

bool TaskManager::Runnable() const {
    for(std::size_t i=0;i<queue_.size();++i)
        if(!config_running_||entries_.at(queue_[i]).task.type!="router_config") return true;
    return false;
}

void TaskManager::Run() {
    while (true) {
        ExecTask task;
        std::uint32_t max_payload;
        std::shared_ptr<std::atomic<bool>> cancel;
        {
            std::unique_lock<std::mutex> lock(mutex_);
            condition_.wait(lock, [this] { return stop_.load() || Runnable(); });
            if (stop_.load()) return;
            std::deque<std::string>::iterator next=queue_.begin();
            while(config_running_ && entries_.at(*next).task.type=="router_config") ++next;
            Entry& entry = entries_.at(*next);
            queue_.erase(next);
            if(entry.task.type=="router_config") config_running_=true;
            entry.state = "running";
            task = entry.task;cancel=entry.cancel;
            max_payload = entry.max_payload;
            running_.fetch_add(1);
            std::cout << "task_state=RUNNING task_id=" << task.task_id << std::endl;
        }
        ExecResult result;
        if(task.type=="neighbor_scan"){
            if(neighbors_)result=neighbors_->Scan(task,cancel);
            else{result.task_id=task.task_id;result.status="failed";result.stderr_text="neighbors_unavailable";result.started_at=result.finished_at=static_cast<std::uint64_t>(std::time(NULL));}
        }else result=ExecuteExec(task,&stop_);
        std::string payload, error;
        // Valid task IDs and >=1024-byte negotiation guarantee metadata fits.
        if (!BuildTaskResultPayload(result, max_payload, &payload, &error)) {
            result.stdout_text.clear();
            result.stderr_text.clear();
            result.truncated = true;
            BuildTaskResultPayload(result, max_payload, &payload, &error);
        }
        {
            std::lock_guard<std::mutex> lock(mutex_);
            Entry& entry = entries_.at(task.task_id);
            entry.state = result.status;
            reserved_bytes_ -= entry.max_payload - payload.size();
            entry.payload.swap(payload);
            running_.fetch_sub(1);
            if(task.type=="router_config") config_running_=false;
            condition_.notify_all();
            std::cout << "task_state=" << result.status << " task_id=" << task.task_id << std::endl;
        }
    }
}
}
