#ifndef RMP_IDENTITY_H
#define RMP_IDENTITY_H

#include <string>

namespace rmp {
// Resolve once before connecting. An explicit ID never executes a command.
bool ResolveDeviceId(bool explicit_id, std::string* device_id, std::string* error);
}

#endif
