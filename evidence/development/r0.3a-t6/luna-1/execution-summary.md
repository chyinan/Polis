# R0.3A-T6 Minimal Native Transport Canary

Result: `INCONCLUSIVE`.

The canary used the same NativeArgs/app-server/client wiring and runtime category as T3, but registered zero dynamic Polis tools, zero schema bytes, and no employee/business binding. Readiness was recorded before `turn/start`.

The native turn reached `turn/started`, then produced five structured `responseStreamDisconnected` events with `willRetry=true` in `pre_first_output_reconnecting`. No valid model output, usage update, tool call, or business side effect occurred. The phase-aware first-output deadline then expired at approximately 90 seconds; stop proof was confirmed. The sentinel was not matched because no model output arrived.

This means the current execution combination cannot qualify even the minimal zero-business-tool transport canary. It does not prove a permanent provider defect, but it does qualify the current environment as `native_execution_environment_unqualified` for further R0.3A business execution. No retry or second canary was started.
