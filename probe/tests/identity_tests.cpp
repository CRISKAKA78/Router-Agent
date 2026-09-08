#include "rmp/identity.h"
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <sys/stat.h>
#include <unistd.h>

int main() {
    char directory[] = "/tmp/rmp-identity-XXXXXX";
    if (!mkdtemp(directory)) return 1;
    const std::string script = std::string(directory) + "/nvram";
    const char* old = std::getenv("PATH");
    const std::string path = old ? old : "";
    setenv("PATH", directory, 1);
    int failures = 0;
    const auto check = [&](const std::string& body, bool explicit_id,
                           const std::string& initial, bool success, const std::string& expected) {
        { std::ofstream file(script.c_str()); file << "#!/bin/sh\n[ \"$1\" = get ] && [ \"$2\" = SN ] || exit 9\n" << body << '\n'; }
        chmod(script.c_str(), 0700);
        std::string id = initial, error;
        const bool result = rmp::ResolveDeviceId(explicit_id, &id, &error);
        if (result != success || (success && id != expected) || (!success && error.empty())) {
            std::cerr << "identity case failed: " << body << " error=" << error << '\n';
            ++failures;
        }
    };
    check("printf '  SN-123 \\r\\n'", false, "", true, "SN-123");
    check("exit 8", true, "manual-id", true, "manual-id");
    check("printf SN", true, "", false, "");
    check("printf SN; exit 1", false, "", false, "");
    check("printf ' \\n'", false, "", false, "");
    check("printf 'one\\ntwo'", false, "", false, "");
    check("printf '\\377'", false, "", false, "");
    check("printf 'a\\000b'", false, "", false, "");
    check("printf '%0129d' 0", false, "", false, "");
    check("printf '%0128d' 0", false, "", true, std::string(128, '0'));
    check("exec /bin/sleep 20", false, "", false, "");
    unlink(script.c_str());
    std::string id, error;
    if (rmp::ResolveDeviceId(false, &id, &error)) ++failures;
    setenv("PATH", path.c_str(), 1);
    rmdir(directory);
    return failures == 0 ? 0 : 1;
}
