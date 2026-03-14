---
name: platformio-windows-ops
description: Handle Stackchan firmware build/upload operations on Windows with PlatformIO, including pio command fallback to py -m platformio, PATH diagnosis, proxy/TLS certificate errors, and safe recovery steps. Use when build, upload, or PlatformIO environment setup fails.
---

# PlatformIO Windows Ops

Use this skill when requests involve firmware build or upload on Windows and any of the following appears:
- `pio` command is not found
- `py -m platformio run` succeeds while `pio` fails
- TLS/certificate/proxy errors during package download
- Board upload troubleshooting for CoreS3

## Scope

In scope:
- Diagnose PlatformIO CLI availability on Windows.
- Standardize command usage for this repository.
- Resolve proxy/TLS download failures with documented temporary workaround.
- Verify build success and device detection for upload readiness.
- Provide safe next steps for upload.

Out of scope:
- Corporate certificate issuance workflows.
- Rewriting firmware architecture.
- Non-Windows development environment support as primary path.

## Repository Policy

For this repository on Windows, prefer this command form:
- `py -m platformio ...`

Rationale:
- `pip install --user platformio` often installs scripts under user AppData path not included in PATH.
- `py -m` avoids PATH dependency and is deterministic.

## Quick Diagnostic Checklist

1. Confirm CLI availability:
- `Get-Command pio -ErrorAction SilentlyContinue`
- `py --version`
- `py -m platformio --version`

2. Confirm Scripts path in PATH:
- expected scripts path example:
  - `C:\Users\<user>\AppData\Roaming\Python\Python313\Scripts`

3. Confirm build prerequisites:
- `firmware/include/secrets.h` exists
- run build:
  - `py -m platformio run -e stackchan_cores3`

4. Confirm upload prerequisites:
- `py -m platformio device list`
- detect expected ESP32 serial device (e.g. VID:PID `303A:1001`)

## TLS / Proxy Failure Playbook

Typical failure indicators:
- `self-signed certificate in certificate chain`
- `SSL: CERTIFICATE_VERIFY_FAILED`
- `HTTPClientError` while installing tools/frameworks

Temporary workaround (development only):
1. `py -m platformio settings set enable_proxy_strict_ssl No`
2. re-run build:
   - `py -m platformio run -e stackchan_cores3`

Permanent remediation (recommended):
- install corporate root CA correctly for OS/Python trust chain
- revert strict setting:
  - `py -m platformio settings set enable_proxy_strict_ssl Yes`

Operational rule:
- keep `enable_proxy_strict_ssl Yes` for CI/production
- treat `No` as temporary local workaround

## PATH Guidance (Optional)

If user prefers `pio` command:
- add Python Scripts directory to user PATH
- open a new shell session
- verify with `pio --version`

Important:
- restart alone does not add missing PATH entries
- PATH change is required before restart/new shell takes effect

## Standard Commands

Build:
- `py -m platformio run -e stackchan_cores3`

Upload:
- `py -m platformio run -e stackchan_cores3 -t upload --upload-port <COMx>`

Monitor:
- `py -m platformio device monitor --baud 115200`

## Expected Deliverable Format

Return in this order:
1) Environment diagnosis summary
2) Build status (pass/fail with primary reason)
3) Upload readiness status (device detected or not)
4) Applied workaround and rollback note
5) Exact next command to run

## References

- docs/project/secrets-operations.md
- firmware/README.md
- firmware/platformio.ini
