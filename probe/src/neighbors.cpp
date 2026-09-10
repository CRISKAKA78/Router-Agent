#include "rmp/neighbors.h"
#include "rmp/json.h"
#include <algorithm>
#include <arpa/inet.h>
#include <cctype>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <ctime>
#include <fcntl.h>
#include <fstream>
#include <linux/if_ether.h>
#include <linux/if_packet.h>
#include <linux/neighbour.h>
#include <linux/netlink.h>
#include <linux/rtnetlink.h>
#include <net/if.h>
#include <poll.h>
#include <set>
#include <sstream>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <unistd.h>
namespace rmp {
namespace {
using Clock = std::chrono::steady_clock;
bool Name(const std::string &s, unsigned limit = 15) {
  return !s.empty() && s.size() <= limit && s != "." && s != ".." &&
         s.find_first_not_of("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVW"
                             "XYZ0123456789_.-") == std::string::npos;
}
bool ID(const std::string &s) {
  return !s.empty() && s.size() <= 32 && s[0] >= 'a' && s[0] <= 'z' &&
         s.find_first_not_of("abcdefghijklmnopqrstuvwxyz0123456789_") ==
             std::string::npos;
}
std::string Str(const JsonObject &o, const std::string &k) {
  auto i = o.find(k);
  return i != o.end() && i->second.type == JsonType::kString
             ? i->second.string_value
             : "";
}
bool Text(const std::string &s, unsigned max) {
  JsonObject o;
  std::string e;
  return s.size() <= max && s.find_first_of("\r\n") == std::string::npos &&
         s.find('\0') == std::string::npos &&
         ParseJsonObject("{\"v\":" + EscapeJsonString(s) + "}", &o, &e);
}
std::vector<JsonObject> Objects(const std::string &s) {
  std::vector<JsonObject> out;
  bool quote = false, escape = false;
  int depth = 0;
  std::size_t start = 0;
  if (s.empty() || s.front() != '[' || s.back() != ']')
    return out;
  for (std::size_t i = 0; i < s.size(); ++i) {
    char c = s[i];
    if (quote) {
      if (escape)
        escape = false;
      else if (c == '\\')
        escape = true;
      else if (c == '"')
        quote = false;
      continue;
    }
    if (depth == 0 && c != '{' && c != '[' && c != ']' && c != ',' &&
        !std::isspace(static_cast<unsigned char>(c)))
      return {};
    if (c == '"') {
      quote = true;
      continue;
    }
    if (c == '{') {
      if (depth++ == 0)
        start = i;
    } else if (c == '}' && --depth == 0) {
      JsonObject o;
      std::string e;
      if (!ParseJsonObject(s.substr(start, i - start + 1), &o, &e) ||
          out.size() >= 8)
        return {};
      out.push_back(o);
    }
  }
  return depth == 0 && !quote ? out : std::vector<JsonObject>{};
}
std::string Read(const std::string &p, bool *ok = NULL) {
  std::ifstream f(p.c_str(), std::ios::binary);
  if (ok)
    *ok = false;
  if (!f)
    return "";
  std::string s(131073, '\0');
  f.read(&s[0], s.size());
  s.resize(static_cast<std::size_t>(f.gcount()));
  if (s.size() > 131072)
    return "";
  if (ok)
    *ok = true;
  return s;
}
std::string Mac(const unsigned char *b) {
  char s[18];
  std::snprintf(s, sizeof(s), "%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2],
                b[3], b[4], b[5]);
  return s;
}
std::string NormalizeMac(const std::string &s) {
  unsigned v[6];
  char tail;
  if (std::sscanf(s.c_str(), "%2x:%2x:%2x:%2x:%2x:%2x%c", &v[0], &v[1], &v[2],
                  &v[3], &v[4], &v[5], &tail) != 6 ||
      s.size() != 17 || v[0] & 1)
    return "";
  unsigned char b[6];
  unsigned sum = 0;
  for (int i = 0; i < 6; ++i) {
    b[i] = static_cast<unsigned char>(v[i]);
    sum += v[i];
  }
  return sum ? Mac(b) : "";
}
std::string IP(const void *p, int family) {
  char out[INET6_ADDRSTRLEN];
  return inet_ntop(family, p, out, sizeof(out)) ? out : "";
}
std::string Interface(int index) {
  char name[IF_NAMESIZE];
  return if_indextoname(index, name) ? name : "";
}
std::string Master(const std::string &root, const std::string &name) {
  char b[512];
  auto n = readlink((root + "/sys/class/net/" + name + "/master").c_str(), b,
                    sizeof(b) - 1);
  if (n <= 0)
    return "";
  b[n] = 0;
  std::string s = b;
  return s.substr(s.find_last_of('/') + 1);
}
int Socket(int family, int type, int proto) {
  std::lock_guard<std::mutex> g(ExecForkMutex());
  int fd = socket(family, type, proto);
  if (fd >= 0) {
    if (fcntl(fd, F_SETFD, FD_CLOEXEC) < 0 ||
        fcntl(fd, F_SETFL, O_NONBLOCK) < 0) {
      close(fd);
      return -1;
    }
  }
  return fd;
}
struct FD {
  int n;
  explicit FD(int value) : n(value) {}
  ~FD() {
    if (n >= 0)
      close(n);
  }
};
bool Address(const std::string &iface, std::uint32_t *ip, std::uint32_t *mask,
             unsigned char *mac = NULL) {
  FD fd(Socket(AF_INET, SOCK_DGRAM, 0));
  if (fd.n < 0)
    return false;
  struct ifreq r = {};
  std::strncpy(r.ifr_name, iface.c_str(), IFNAMSIZ - 1);
  if (ioctl(fd.n, SIOCGIFADDR, &r) < 0)
    return false;
  *ip = ntohl(reinterpret_cast<sockaddr_in *>(&r.ifr_addr)->sin_addr.s_addr);
  if (ioctl(fd.n, SIOCGIFNETMASK, &r) < 0)
    return false;
  *mask =
      ntohl(reinterpret_cast<sockaddr_in *>(&r.ifr_netmask)->sin_addr.s_addr);
  if (mac) {
    if (ioctl(fd.n, SIOCGIFHWADDR, &r) < 0 || r.ifr_hwaddr.sa_family != 1)
      return false;
    std::memcpy(mac, r.ifr_hwaddr.sa_data, 6);
  }
  return true;
}
struct Learned {
  NeighborRow row;
  std::string iface, bridge;
  unsigned vlan = 0;
};
std::vector<Learned> Netlink(const std::string &root, int family,
                             const std::atomic<bool> *cancel, bool *ok) {
  *ok = false;
  std::vector<Learned> out;
  FD fd(Socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE));
  if (fd.n < 0)
    return out;
  struct {
    nlmsghdr h;
    ndmsg n;
  } request = {};
  request.h.nlmsg_len = NLMSG_LENGTH(sizeof(ndmsg));
  request.h.nlmsg_type = RTM_GETNEIGH;
  request.h.nlmsg_flags = NLM_F_REQUEST | NLM_F_DUMP;
  request.h.nlmsg_seq = 1;
  request.n.ndm_family = static_cast<unsigned char>(family);
  sockaddr_nl kernel = {};
  kernel.nl_family = AF_NETLINK;
  if (sendto(fd.n, &request, request.h.nlmsg_len, 0,
             reinterpret_cast<sockaddr *>(&kernel), sizeof(kernel)) < 0)
    return out;
  auto until = Clock::now() + std::chrono::milliseconds(500);
  std::size_t bytes = 0;
  while (Clock::now() < until && !cancel->load()) {
    pollfd p = {fd.n, POLLIN, 0};
    if (poll(&p, 1, 20) <= 0)
      continue;
    alignas(nlmsghdr) char buffer[32768];
    sockaddr_nl from = {};
    socklen_t length = sizeof(from);
    int size = static_cast<int>(
        recvfrom(fd.n, buffer, sizeof(buffer), MSG_TRUNC,
                 reinterpret_cast<sockaddr *>(&from), &length));
    if (size <= 0 || size > static_cast<int>(sizeof(buffer)) ||
        from.nl_pid != 0)
      return out;
    bytes += size;
    if (bytes > 1048576)
      return out;
    for (auto h = reinterpret_cast<nlmsghdr *>(buffer); NLMSG_OK(h, size);
         h = NLMSG_NEXT(h, size)) {
      if (h->nlmsg_seq != 1)
        continue;
      if (h->nlmsg_type == NLMSG_DONE) {
        *ok = (h->nlmsg_flags & NLM_F_DUMP_INTR) == 0;
        return out;
      }
      if (h->nlmsg_type == NLMSG_ERROR)
        return out;
      if (h->nlmsg_type != RTM_NEWNEIGH ||
          h->nlmsg_len < NLMSG_LENGTH(sizeof(ndmsg)))
        continue;
      auto n = reinterpret_cast<ndmsg *>(NLMSG_DATA(h));
      if (n->ndm_family != family ||
          n->ndm_state & (NUD_INCOMPLETE | NUD_FAILED))
        continue;
      if (family == AF_BRIDGE && (n->ndm_state & (NUD_NOARP | NUD_PERMANENT)))
        continue;
      Learned v;
      v.iface = Interface(n->ndm_ifindex);
      v.bridge = Master(root, v.iface);
      v.row.state = n->ndm_state & NUD_REACHABLE ? "reachable" : "cached";
      v.row.source = family == AF_INET    ? "arp"
                     : family == AF_INET6 ? "ndp"
                                          : "fdb";
      int remaining =
          static_cast<int>(h->nlmsg_len) - NLMSG_LENGTH(sizeof(ndmsg));
      for (auto a = reinterpret_cast<rtattr *>(reinterpret_cast<char *>(n) +
                                               NLMSG_ALIGN(sizeof(ndmsg)));
           RTA_OK(a, remaining); a = RTA_NEXT(a, remaining)) {
        if (a->rta_type == NDA_LLADDR && RTA_PAYLOAD(a) == 6)
          v.row.mac =
              NormalizeMac(Mac(reinterpret_cast<unsigned char *>(RTA_DATA(a))));
        if (a->rta_type == NDA_DST &&
            RTA_PAYLOAD(a) == (family == AF_INET ? 4 : 16))
          v.row.ip = IP(RTA_DATA(a), family);
        if (a->rta_type == 5 && RTA_PAYLOAD(a) == 2) {
          unsigned short vid;
          std::memcpy(&vid, RTA_DATA(a), 2);
          v.vlan = vid;
        }
        if (a->rta_type == 9 && RTA_PAYLOAD(a) == 4) {
          int index;
          std::memcpy(&index, RTA_DATA(a), 4);
          v.bridge = Interface(index);
        }
      }
      if (!v.row.mac.empty() && Name(v.iface) && out.size() < 2048)
        out.push_back(v);
      else if (out.size() >= 2048)
        return out;
    }
  }
  return out;
}
void Merge(std::vector<NeighborRow> &rows, const NeighborRow &row) {
  if (row.mac.empty())
    return;
  for (auto &i : rows)
    if (i.mac == row.mac && i.ip == row.ip) {
      if (i.source.find(row.source) == std::string::npos)
        i.source += "+" + row.source;
      if (!row.port.empty())
        i.port = row.port;
      if (!row.hostname.empty())
        i.hostname = row.hostname;
      if (row.state == "responded" || row.state == "reachable")
        i.state = row.state;
      return;
    }
  if (rows.size() < 512)
    rows.push_back(row);
}
std::string RowJSON(const NeighborRow &r) {
  return "{\"interface\":" + EscapeJsonString(r.interface) +
         ",\"ip\":" + EscapeJsonString(r.ip) +
         ",\"mac\":" + EscapeJsonString(r.mac) +
         ",\"port\":" + EscapeJsonString(r.port) +
         ",\"hostname\":" + EscapeJsonString(r.hostname) +
         ",\"source\":" + EscapeJsonString(r.source) +
         ",\"state\":" + EscapeJsonString(r.state) + "}";
}
std::string DomainIssue(const std::string &root, const NeighborDomain &d) {
  if (root.empty() && if_nametoindex(d.interface.c_str()) == 0)
    return "interface_missing";
  if (!Master(root, d.interface).empty())
    return "bridge_member_use_master";
  auto filtering =
      Read(root + "/sys/class/net/" + d.interface + "/bridge/vlan_filtering");
  if (!filtering.empty() && filtering[0] == '1')
    return "vlan_bridge_requires_l3_interface";
  auto type = Read(root + "/sys/class/net/" + d.interface + "/type");
  if (!type.empty() && type != "1\n" && type != "1")
    return "not_ethernet";
  return "";
}
} // namespace
bool NeighborRange(const std::string &s, std::uint32_t *first,
                   std::uint32_t *last) {
  auto slash = s.find('/');
  if (slash == std::string::npos)
    return false;
  std::string bits = s.substr(slash + 1);
  if (bits.empty() || bits.size() > 2 ||
      bits.find_first_not_of("0123456789") != std::string::npos)
    return false;
  unsigned prefix = static_cast<unsigned>(std::strtoul(bits.c_str(), NULL, 10));
  in_addr a;
  if (prefix < 24 || prefix > 32 ||
      inet_pton(AF_INET, s.substr(0, slash).c_str(), &a) != 1)
    return false;
  std::uint32_t mask = 0xffffffffU << (32 - prefix);
  *first = ntohl(a.s_addr);
  *last = *first | ~mask;
  return (*first & mask) == *first;
}
bool ParseNeighborTask(const std::string &s, bool cancel,
                       RouterConfigParams *out) {
  JsonObject o;
  std::string e;
  if (!ParseJsonObject(s, &o, &e))
    return false;
  out->clear();
  if (cancel) {
    auto id = Str(o, "target_task_id");
    if (o.size() != 1 || id.empty() || id.size() > 128)
      return false;
    (*out)["target_task_id"] = id;
    return true;
  }
  std::uint32_t first, last;
  auto rev = o.find("config_revision");
  if (o.size() != 3 || !ID(Str(o, "domain_id")) ||
      !NeighborRange(Str(o, "cidr"), &first, &last) || rev == o.end() ||
      rev->second.type != JsonType::kUnsignedInteger ||
      !rev->second.unsigned_value)
    return false;
  (*out)["domain_id"] = Str(o, "domain_id");
  (*out)["cidr"] = Str(o, "cidr");
  (*out)["config_revision"] = std::to_string(rev->second.unsigned_value);
  return true;
}
bool ParseNeighborPlan(const std::string &s, NeighborPlan *out) {
  NeighborPlan p;
  JsonObject o;
  std::string e;
  if (!ParseJsonObject(s, &o, &e) || !o.count("domains"))
    return false;
  for (const auto &i : o)
    if (i.first != "domains" && i.first != "interval_seconds" &&
        i.first != "fdb_command")
      return false;
  if (o.count("interval_seconds")) {
    auto n = o["interval_seconds"];
    if (n.type != JsonType::kUnsignedInteger || n.unsigned_value < 10 ||
        n.unsigned_value > 86400)
      return false;
    p.interval = static_cast<unsigned>(n.unsigned_value);
  }
  if (o.count("fdb_command")) {
    if (o["fdb_command"].type != JsonType::kString)
      return false;
    p.fdb_command = Str(o, "fdb_command");
    if (p.fdb_command.size() > 4096 ||
        p.fdb_command.find('\0') != std::string::npos)
      return false;
  }
  std::set<std::string> ids, interfaces;
  for (const auto &v : Objects(o["domains"].raw_value)) {
    for (const auto &i : v)
      if (i.first != "id" && i.first != "scope" && i.first != "interface" &&
          i.first != "lease_file" && i.first != "ports")
        return false;
    NeighborDomain d;
    d.id = Str(v, "id");
    d.scope = Str(v, "scope");
    d.interface = Str(v, "interface");
    d.lease_file = Str(v, "lease_file");
    if (!ID(d.id) || (d.scope != "lan" && d.scope != "broadcast") ||
        !Name(d.interface) || !ids.insert(d.id).second)
      return false;
    if (v.count("lease_file") &&
        (v.at("lease_file").type != JsonType::kString ||
         (!d.lease_file.empty() &&
          (d.lease_file[0] != '/' || !Text(d.lease_file, 256)))))
      return false;
    if (v.count("ports")) {
      auto raw = v.at("ports").raw_value;
      if (raw.empty() || raw.front() != '[')
        return false;
      bool quote = false, escape = false;
      std::size_t start = 0;
      for (std::size_t i = 0; i < raw.size(); ++i) {
        char c = raw[i];
        if (quote) {
          if (escape)
            escape = false;
          else if (c == '\\')
            escape = true;
          else if (c == '"') {
            quote = false;
            JsonObject item;
            std::string error;
            if (!ParseJsonObject("{\"p\":" + raw.substr(start, i - start + 1) +
                                     "}",
                                 &item, &error))
              return false;
            auto port = Str(item, "p");
            if (port.empty() || !Text(port, 128) || d.ports.size() >= 64 ||
                std::find(d.ports.begin(), d.ports.end(), port) !=
                    d.ports.end())
              return false;
            d.ports.push_back(port);
          }
        } else if (c == '"') {
          quote = true;
          start = i;
        } else if (c != '[' && c != ']' && c != ',' && c != ' ' && c != '\n' &&
                   c != '\r' && c != '\t')
          return false;
      }
    }
    if (d.scope == "lan" && d.ports.empty())
      return false;
    for (const auto &other : p.domains)
      if (other.interface == d.interface) {
        if (d.scope == "broadcast" || other.scope == "broadcast") {
          if (d.scope == other.scope)
            return false;
          continue;
        }
        if (other.ports.empty() || d.ports.empty())
          return false;
        for (const auto &port : d.ports)
          if (std::find(other.ports.begin(), other.ports.end(), port) !=
              other.ports.end())
            return false;
      }
    p.domains.push_back(d);
  }
  if (p.domains.empty())
    return false;
  *out = p;
  return true;
}
std::vector<NeighborRow> ParseARP(const std::string &s,
                                  const std::string &iface) {
  std::vector<NeighborRow> out;
  std::istringstream lines(s);
  std::string line;
  unsigned count = 0;
  while (std::getline(lines, line) && count++ < 2048) {
    std::istringstream fields(line);
    std::string ip, type, flags, mac, mask, dev;
    fields >> ip >> type >> flags >> mac >> mask >> dev;
    in_addr a;
    if (dev != iface || inet_pton(AF_INET, ip.c_str(), &a) != 1 ||
        (std::strtoul(flags.c_str(), NULL, 0) & 2) == 0)
      continue;
    NeighborRow row;
    row.ip = ip;
    row.mac = NormalizeMac(mac);
    row.source = "arp";
    row.state = "cached";
    Merge(out, row);
  }
  return out;
}
Neighbors::Neighbors(const std::string &root) : root_(root) {
  worker_ = std::thread(&Neighbors::Run, this);
}
Neighbors::~Neighbors() {
  stop_ = true;
  Configure("", 0);
  if (worker_.joinable())
    worker_.join();
}
void Neighbors::Configure(const std::string &json, std::uint64_t rev) {
  NeighborPlan plan;
  if (!json.empty() && !ParseNeighborPlan(json, &plan))
    return;
  std::lock_guard<std::mutex> g(mutex_);
  if (json_ == json && revision_ == rev)
    return;
  if (collecting_)
    *collecting_ = true;
  if (active_)
    *active_ = true;
  json_ = json;
  plan_ = plan;
  revision_ = json.empty() ? 0 : rev;
  recent_.clear();
  latest_.clear();
  pending_ = false;
  refresh_ = true;
}
bool Neighbors::Next(std::size_t limit, std::string *out) {
  std::lock_guard<std::mutex> g(mutex_);
  if (!pending_ || latest_.empty())
    return false;
  *out = "{\"event\":\"neighbors\",\"config_revision\":" +
         std::to_string(revision_) + ",\"age_ms\":" +
         std::to_string(std::chrono::duration_cast<std::chrono::milliseconds>(
                            Clock::now() - sampled_)
                            .count()) +
         "," + latest_ + "}";
  if (out->size() > limit)
    return false;
  pending_ = false;
  return true;
}
void Neighbors::Run() {
  auto due = Clock::now();
  while (!stop_) {
    NeighborPlan plan;
    std::uint64_t rev;
    std::vector<Recent> recent;
    std::shared_ptr<std::atomic<bool>> cancel;
    {
      std::lock_guard<std::mutex> g(mutex_);
      rev = revision_;
      if (rev && (refresh_ || Clock::now() >= due)) {
        plan = plan_;
        recent = recent_;
        refresh_ = false;
        collecting_ = std::make_shared<std::atomic<bool>>(false);
        cancel = collecting_;
      }
    }
    if (!cancel) {
      std::this_thread::sleep_for(std::chrono::milliseconds(50));
      continue;
    }
    bool arpOK = false, ipv4OK = false, ndpOK = false, fdbOK = false;
    auto arp = Read(root_ + "/proc/net/arp", &arpOK);
    auto ipv4 = Netlink(root_, AF_INET, cancel.get(), &ipv4OK);
    auto ndp = Netlink(root_, AF_INET6, cancel.get(), &ndpOK);
    auto fdb = Netlink(root_, AF_BRIDGE, cancel.get(), &fdbOK);
    std::string vendor;
    bool vendorOK = true;
    if (!plan.fdb_command.empty()) {
      ExecTask t;
      t.type = "exec";
      t.command = plan.fdb_command;
      t.timeout = 5;
      auto result = ExecuteExec(t, cancel.get());
      vendorOK = result.status == "success" && !result.truncated &&
                 result.stdout_text.size() <= 32768;
      if (vendorOK)
        vendor = result.stdout_text;
    }
    std::string domains;
    unsigned total = 0;
    std::size_t encoded = 0;
    bool snapshotLimited = false;
    std::map<std::string, NeighborRow> unclassified;
    std::set<std::string> classified;
    for (const auto &d : plan.domains) {
      std::string reason = DomainIssue(root_, d);
      std::vector<NeighborRow> rows;
      bool limited = false;
      if (reason.empty()) {
        rows = ParseARP(arp, d.interface);
        for (const auto &v : ipv4)
          if (v.iface == d.interface)
            Merge(rows, v.row);
        for (const auto &v : ndp)
          if (v.iface == d.interface)
            Merge(rows, v.row);
        for (const auto &r : recent)
          if (r.row.interface == d.interface &&
              Clock::now() - r.at < std::chrono::seconds(60))
            Merge(rows, r.row);
        if (!arpOK && !ipv4OK)
          reason = "arp_unavailable";
        if (!ndpOK)
          reason +=
              (reason.empty() ? "" : ";") + std::string("ndp_unavailable");
        if (!d.lease_file.empty()) {
          bool leaseOK = false;
          auto leases = Read(root_ + d.lease_file, &leaseOK);
          if (!leaseOK)
            reason +=
                (reason.empty() ? "" : ";") + std::string("leases_unavailable");
          std::uint32_t local = 0, mask = 0;
          bool subnet = Address(d.interface, &local, &mask);
          std::istringstream lines(leases);
          std::string line;
          while (std::getline(lines, line)) {
            std::istringstream fields(line);
            std::string expiry, mac, ip, host;
            fields >> expiry >> mac >> ip >> host;
            in_addr a;
            auto normalized = NormalizeMac(mac);
            if (expiry.empty() ||
                expiry.find_first_not_of("0123456789") != std::string::npos ||
                normalized.empty() || inet_pton(AF_INET, ip.c_str(), &a) != 1 ||
                !Text(host, 128))
              continue;
            auto end = std::strtoull(expiry.c_str(), NULL, 10);
            if (end && end <= static_cast<unsigned long long>(std::time(NULL)))
              continue;
            bool matched = false;
            for (const auto &r : rows)
              if (r.ip == ip && r.mac == normalized)
                matched = true;
            if (!matched &&
                (!subnet || (ntohl(a.s_addr) & mask) != (local & mask)))
              continue;
            NeighborRow r;
            r.ip = ip;
            r.mac = normalized;
            r.hostname = host == "*" ? "" : host;
            r.source = "dhcp";
            r.state = "lease";
            Merge(rows, r);
          }
        }
        std::map<std::string, std::set<std::string>> ports;
        if (access(
                (root_ + "/sys/class/net/" + d.interface + "/bridge").c_str(),
                F_OK) != 0) {
          for (auto &r : rows)
            r.port = d.interface;
        }
        std::string bridge = d.interface;
        unsigned vlan = 0;
        auto vlanText = Read(root_ + "/proc/net/vlan/" + d.interface);
        auto vid = vlanText.find("VID:");
        auto dev = vlanText.find("Device:");
        if (vid != std::string::npos && dev != std::string::npos) {
          vlan = std::strtoul(vlanText.c_str() + vid + 4, NULL, 10);
          std::istringstream in(vlanText.substr(dev + 7));
          in >> bridge;
        }
        for (const auto &v : fdb)
          if (v.bridge == bridge && v.vlan == vlan)
            ports[v.row.mac].insert(v.iface);
        std::map<std::string, std::set<std::string>> vendorPorts;
        std::istringstream vendorRows(vendor);
        std::string line;
        while (std::getline(vendorRows, line)) {
          std::istringstream fields(line);
          std::string id, mac, port, extra;
          std::getline(fields, id, '\t');
          std::getline(fields, mac, '\t');
          std::getline(fields, port, '\t');
          if (id == d.id && !NormalizeMac(mac).empty() && !port.empty() &&
              Text(port, 128) && !std::getline(fields, extra, '\t'))
            vendorPorts[NormalizeMac(mac)].insert(port);
        }
        for (const auto &p : vendorPorts)
          ports[p.first] = p.second;
        for (const auto &p : ports) {
          bool found = false;
          for (auto &r : rows)
            if (r.mac == p.first) {
              found = true;
              r.port = p.second.size() == 1 ? *p.second.begin() : "";
              if (r.source.find("fdb") == std::string::npos)
                r.source += "+fdb";
            }
          if (!found) {
            NeighborRow r;
            r.mac = p.first;
            r.source = "fdb";
            r.state = "mac_only";
            if (p.second.size() == 1)
              r.port = *p.second.begin();
            Merge(rows, r);
          }
        }
        if (!fdbOK || !vendorOK)
          reason +=
              (reason.empty() ? "" : ";") +
              std::string(!vendorOK ? "fdb_command_failed" : "fdb_unavailable");
      }
      auto self = Read(root_ + "/sys/class/net/" + d.interface + "/address");
      if (!self.empty() && self.back() == '\n')
        self.pop_back();
      rows.erase(
          std::remove_if(rows.begin(), rows.end(),
                         [&](const NeighborRow &r) { return r.mac == self; }),
          rows.end());
      if (d.scope == "lan") {
        std::vector<NeighborRow> selected;
        for (auto row : rows) {
          auto key = d.interface + "/" + row.ip + "/" + row.mac;
          if (!row.port.empty() && std::find(d.ports.begin(), d.ports.end(),
                                             row.port) != d.ports.end()) {
            selected.push_back(row);
            classified.insert(key);
          } else {
            row.interface = d.interface;
            unclassified[key] = row;
          }
        }
        rows.swap(selected);
      }
      std::sort(rows.begin(), rows.end(),
                [](const NeighborRow &a, const NeighborRow &b) {
                  return a.ip == b.ip ? a.mac < b.mac : a.ip < b.ip;
                });
      std::string items;
      for (const auto &r : rows) {
        auto json = RowJSON(r);
        if (total >= 256 || encoded + json.size() > 45000) {
          limited = true;
          break;
        }
        if (!items.empty())
          items += ",";
        items += json;
        encoded += json.size();
        ++total;
      }
      if (rows.size() >= 512)
        limited = true;
      snapshotLimited = snapshotLimited || limited;
      if (!domains.empty())
        domains += ",";
      std::string status = reason.empty() ? "ok"
                           : rows.empty() ? "error"
                                          : "partial";
      domains += "{\"id\":" + EscapeJsonString(d.id) +
                 ",\"scope\":" + EscapeJsonString(d.scope) +
                 ",\"interface\":" + EscapeJsonString(d.interface) +
                 ",\"status\":" + EscapeJsonString(status) +
                 ",\"reason\":" + EscapeJsonString(reason) +
                 ",\"limited\":" + (limited ? "true" : "false") +
                 ",\"rows\":[" + items + "]}";
    }
    std::string unknown;
    for (const auto &item : unclassified) {
      if (classified.count(item.first))
        continue;
      auto json = RowJSON(item.second);
      if (total >= 256 || encoded + json.size() > 45000) {
        snapshotLimited = true;
        break;
      }
      if (!unknown.empty())
        unknown += ",";
      unknown += json;
      encoded += json.size();
      ++total;
    }
    {
      std::lock_guard<std::mutex> g(mutex_);
      if (revision_ == rev && !cancel->load()) {
        sampled_ = Clock::now();
        latest_ =
            "\"limited\":" + std::string(snapshotLimited ? "true" : "false") +
            ",\"interval_seconds\":" + std::to_string(plan.interval) +
            ",\"domains\":[" + domains + "],\"unclassified\":[" + unknown + "]";
        pending_ = true;
      }
    }
    due = Clock::now() + std::chrono::seconds(plan.interval);
  }
}
ExecResult Neighbors::Scan(const ExecTask &t,
                           const std::shared_ptr<std::atomic<bool>> &cancel) {
  ExecResult result;
  result.task_id = t.task_id;
  result.started_at = static_cast<std::uint64_t>(std::time(NULL));
  result.status = "failed";
  NeighborDomain domain;
  std::uint64_t revision =
      std::strtoull(t.config.at("config_revision").c_str(), NULL, 10);
  auto finish = [&](const std::string &reason) {
    result.stderr_text = reason;
    result.finished_at = static_cast<std::uint64_t>(std::time(NULL));
    std::lock_guard<std::mutex> g(mutex_);
    if (active_ == cancel)
      active_.reset();
    return result;
  };
  {
    std::lock_guard<std::mutex> g(mutex_);
    if (revision_ != revision || active_) {
      result.finished_at = result.started_at;
      result.stderr_text = active_ ? "scan_busy" : "configuration_changed";
      return result;
    }
    for (const auto &d : plan_.domains)
      if (d.id == t.config.at("domain_id"))
        domain = d;
    if (!domain.id.empty())
      active_ = cancel;
  }
  if (domain.id.empty())
    return finish("domain_missing");
  if (cancel->load())
    return finish("cancelled");
  auto issue = DomainIssue(root_, domain);
  if (!issue.empty())
    return finish(issue);
  std::uint32_t first, last, local, mask;
  unsigned char mac[6];
  if (!NeighborRange(t.config.at("cidr"), &first, &last) ||
      !Address(domain.interface, &local, &mask, mac))
    return finish("interface_ipv4_unavailable");
  if ((first & mask) != (local & mask) || (last & mask) != (local & mask))
    return finish("range_not_on_link");
  FD fd(Socket(AF_PACKET, SOCK_RAW, htons(ETH_P_ARP)));
  if (fd.n < 0)
    return finish("raw_socket_unavailable");
  sockaddr_ll bindTo = {};
  bindTo.sll_family = AF_PACKET;
  bindTo.sll_protocol = htons(ETH_P_ARP);
  bindTo.sll_ifindex =
      static_cast<int>(if_nametoindex(domain.interface.c_str()));
  if (bind(fd.n, reinterpret_cast<sockaddr *>(&bindTo), sizeof(bindTo)) < 0)
    return finish("interface_bind_failed");
  sockaddr_ll target = bindTo;
  target.sll_halen = 6;
  std::memset(target.sll_addr, 255, 6);
  unsigned char request[42] = {};
  std::memset(request, 255, 6);
  std::memcpy(request + 6, mac, 6);
  request[12] = 8;
  request[13] = 6;
  request[15] = 1;
  request[16] = 8;
  request[18] = 6;
  request[19] = 4;
  request[21] = 1;
  std::memcpy(request + 22, mac, 6);
  auto source = htonl(local);
  std::memcpy(request + 28, &source, 4);
  auto deadline = Clock::now() + std::chrono::seconds(30),
       sendAt = Clock::now(), doneAt = deadline;
  std::uint64_t next = first;
  std::set<std::uint32_t> sent;
  std::vector<NeighborRow> responses;
  while (Clock::now() < deadline && !cancel->load() && !stop_) {
    auto now = Clock::now();
    if (next <= last && now >= sendAt) {
      auto address = static_cast<std::uint32_t>(next++);
      if (address != local &&
          (last - first < 2 || (address != first && address != last))) {
        auto wire = htonl(address);
        std::memcpy(request + 38, &wire, 4);
        if (sendto(fd.n, request, sizeof(request), 0,
                   reinterpret_cast<sockaddr *>(&target),
                   sizeof(target)) != sizeof(request))
          return finish("arp_send_failed");
        sent.insert(address);
        sendAt = now + std::chrono::milliseconds(63);
      }
      if (next > last)
        doneAt = now + std::chrono::seconds(2);
    }
    if (next > last && now >= doneAt)
      break;
    pollfd p = {fd.n, POLLIN, 0};
    if (poll(&p, 1, 20) <= 0)
      continue;
    unsigned char b[2048];
    sockaddr_ll from = {};
    socklen_t length = sizeof(from);
    auto size = recvfrom(fd.n, b, sizeof(b), 0,
                         reinterpret_cast<sockaddr *>(&from), &length);
    if (size < 42 || from.sll_pkttype == PACKET_OUTGOING ||
        from.sll_ifindex != bindTo.sll_ifindex || b[12] != 8 || b[13] != 6 ||
        b[14] != 0 || b[15] != 1 || b[16] != 8 || b[17] != 0 || b[18] != 6 ||
        b[19] != 4 || b[20] != 0 || b[21] != 2 || std::memcmp(b + 32, mac, 6) ||
        std::memcmp(b + 38, &source, 4) || std::memcmp(b + 6, b + 22, 6))
      continue;
    std::uint32_t peer;
    std::memcpy(&peer, b + 28, 4);
    if (!sent.count(ntohl(peer)))
      continue;
    NeighborRow row;
    row.ip = IP(&peer, AF_INET);
    row.mac = NormalizeMac(Mac(b + 22));
    row.interface = domain.interface;
    row.source = "active_arp";
    row.state = "responded";
    Merge(responses, row);
  }
  if (cancel->load() || stop_)
    return finish("cancelled");
  if (Clock::now() >= deadline) {
    result.status = "timeout";
    return finish("scan_timeout");
  }
  {
    std::lock_guard<std::mutex> g(mutex_);
    if (revision_ != revision) {
      result.stderr_text = "configuration_changed";
    } else {
      auto now = Clock::now();
      recent_.erase(std::remove_if(recent_.begin(), recent_.end(),
                                   [&](const Recent &r) {
                                     return now - r.at >=
                                                std::chrono::seconds(60) ||
                                            r.domain == domain.id;
                                   }),
                    recent_.end());
      for (const auto &r : responses)
        if (recent_.size() < 256)
          recent_.push_back(Recent{domain.id, r, now});
      refresh_ = true;
    }
  }
  if (!result.stderr_text.empty())
    return finish(result.stderr_text);
  result.status = "success";
  result.exit_code = 0;
  result.stdout_text = "{\"requests\":" + std::to_string(sent.size()) +
                       ",\"responses\":" + std::to_string(responses.size()) +
                       "}";
  return finish("");
}
} // namespace rmp
