#include "rmp/router_config.h"
#include "rmp/json.h"

namespace rmp {
namespace {
bool Name(const std::string& s) {
    if (s.empty()) return false;
    for (std::size_t i=0; i<s.size(); ++i) {
        const char c=s[i];
        if (!((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='_')) return false;
    }
    return true;
}
bool UciKey(const std::string& s) {
    if (s.size()>256) return false;
    const std::size_t a=s.find('.'), b=s.find('.', a==std::string::npos?0:a+1);
    if (a==std::string::npos||b==std::string::npos||!Name(s.substr(0,a))||!Name(s.substr(b+1))) return false;
    const std::string section=s.substr(a+1,b-a-1);
    if (Name(section)) return true;
    const std::size_t bracket=section.find('[');
    if (section.empty()||section[0]!='@'||bracket==std::string::npos||section.back()!=']'||!Name(section.substr(1,bracket-1))) return false;
    std::string index=section.substr(bracket+1,section.size()-bracket-2);
    if (!index.empty()&&index[0]=='-') index.erase(0,1);
    if (index.empty()) return false;
    for (std::size_t i=0;i<index.size();++i) if(index[i]<'0'||index[i]>'9') return false;
    return true;
}
bool NvramKey(const std::string& s) {
    if(s.empty()||s.size()>128||s[0]=='-') return false;
    for(std::size_t i=0;i<s.size();++i) if(!Name(s.substr(i,1))&&s[i]!='.'&&s[i]!='/'&&s[i]!=':'&&s[i]!='-') return false;
    return true;
}
}
bool RouterConfigArguments(const RouterConfigParams& p, std::vector<std::string>* args, std::string* error) {
    *error="invalid router configuration parameters";
    if(!p.count("backend")||!p.count("operation")) return false;
    const std::string& backend=p.at("backend"); const std::string& op=p.at("operation");
    if(backend!="nvram"&&backend!="uci") return false;
    std::vector<std::string> out; out.push_back(backend);
    if(op=="commit") {
        if(p.size()!=(backend=="uci"?3U:2U)) return false;
        out.push_back("commit");
        if(backend=="uci") {
            if(!p.count("package")||p.at("package").size()>256||!Name(p.at("package"))) return false;
            out.push_back(p.at("package"));
        }
    } else {
        if(!p.count("key")||!(backend=="nvram"?NvramKey(p.at("key")):UciKey(p.at("key")))) return false;
        if(op=="set") {
            if(p.size()!=4||!p.count("value")||p.at("value").size()>4096||p.at("value").find('\0')!=std::string::npos) return false;
            JsonObject test; std::string ignored;
            if(!ParseJsonObject("{\"v\":"+EscapeJsonString(p.at("value"))+"}",&test,&ignored)) return false;
            out.push_back("set"); out.push_back(p.at("key")+"="+p.at("value"));
        } else {
            if(p.size()!=3||(op!="get"&&op!="delete")) return false;
            out.push_back(op=="delete"&&backend=="nvram"?"unset":op); out.push_back(p.at("key"));
        }
    }
    args->swap(out); error->clear(); return true;
}
bool ParseRouterConfig(const std::string& json, RouterConfigParams* params, std::string* error) {
    JsonObject object; if(!ParseJsonObject(json,&object,error)) return false;
    RouterConfigParams parsed;
    for(JsonObject::const_iterator i=object.begin();i!=object.end();++i) {
        if(i->second.type!=JsonType::kString) { *error="configuration parameters must be strings"; return false; }
        parsed[i->first]=i->second.string_value;
    }
    std::vector<std::string> args;
    if(!RouterConfigArguments(parsed,&args,error)) return false;
    params->swap(parsed); return true;
}
}
