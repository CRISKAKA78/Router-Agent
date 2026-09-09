# Mbed TLS 3.6.6

Source: https://github.com/Mbed-TLS/mbedtls/releases/tag/mbedtls-3.6.6
The include and library directories are unmodified files from the official release archive. The upstream LICENSE is retained. Router-Agent selects the Apache-2.0 licensing option.

The local build uses rmp/tls_config.h for TLS 1.2 client-only HTTPS with ECDHE/AES-GCM and RSA/ECDSA verification. It is statically linked; no target curl/wget, TLS shared library, or certificate package is required.

The embedded public trust roots in rmp/egress_roots.h were obtained on 2026-09-09 from:
- https://cacerts.digicert.com/DigiCertGlobalRootG2.crt.pem
- https://pki.goog/roots.pem

Hostname, certificate chain and time validity are checked. Certificate rotation outside these issuers requires refreshing the embedded roots and rebuilding. DNS resolution uses libc in at most one outstanding background job per IP family; cancellation does not wait on a blocked libc resolver. Requests never invoke a shell.
