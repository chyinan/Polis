# R0.3A-T8 working Codex vs Polis native execution differential

First confirmed divergence: process envelope: Windows installed codex.exe/default user home versus pinned Linux codex under WSL+bwrap with generated home/config

First transport-relevant divergence: A observed proxy environment absent; B/C inject localhost HTTP_PROXY and HTTPS_PROXY from POLIS_NATIVE_PROXY

## Matrix
- executable_path_and_binary: `confirmed_different`; A=e5aa76d19c7c94e2e9ef9b707d590206a73ac0e97c8ddc8382181242494bef75; B=9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a; C=9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a.
- codex_version: `confirmed_different`; A=codex-cli 0.153.4; B=codex-cli 0.151.0; C=codex-cli 0.151.0.
- runtime_profile: `confirmed_different`; A=windows_x64_desktop; B=wsl_linux_amd64_bwrap; C=wsl_linux_amd64_bwrap.
- cwd: `confirmed_different`; A=D:\Programs\Polis; B=/work; C=/work.
- HOME_CODEX_HOME: `confirmed_different`; A=env_absent_default_user_profile; B=explicit_/home/codex; C=explicit_/home/codex.
- config_path_and_digest: `not_recorded/cannot_compare_actual_B_file`; A=95e8b796b32bb68d738982e19bcf31751abb0b37b7195ba9e2dba235bbf4cfbc; B=; C=.
- auth_source_file_fingerprint: `confirmed_same`; A=775b04c0c0249300db19aa61d567c5aa01784d00f6fdf6305d5b7ae5eee5c7f4; B=775b04c0c0249300db19aa61d567c5aa01784d00f6fdf6305d5b7ae5eee5c7f4; C=775b04c0c0249300db19aa61d567c5aa01784d00f6fdf6305d5b7ae5eee5c7f4.
- HTTP_HTTPS_PROXY_env: `confirmed_different`; A=absent_in_observed_shell; B=localhost_proxy_injected; C=localhost_proxy_injected.
- POLIS_NATIVE_PROXY_env: `confirmed_different`; A=absent; B=controller_source_then_HTTP(S)_PROXY_child; C=same_as_B.
- websocket_config: `confirmed_different_configuration`; A=config_or_default; B=explicit_disabled; C=explicit_disabled.
- model_effort_defaults: `confirmed_different`; A=model=gpt-5.6-luna,effort=max; B=model=gpt-5.6-luna,effort=medium; C=model=gpt-5.6-luna,effort=thread-registration-only.
- app_server_stdio: `confirmed_different`; A=app-server observed without --stdio; B=app-server --stdio; C=app-server --stdio.
- HTTP_SSE_fallback_runtime: `not_recorded/cannot_determine_without_live_run`; A=cannot_determine_without_live_run; B=response_stream_used_exact_fallback_not_recorded; C=no_turn.
- OpenAI_Codex_endpoint_override: `confirmed_same_at_manifest_absent`; A=absent; B=absent; C=absent.
- process_ancestry: `not_recorded/cannot_compare_historical_B`; A=captured_current_non_sensitive; B=historical_not_recorded; C=historical_not_recorded.
- callback_local_IPC: `not_recorded/cannot_compare`; A=current process observed but endpoint not recorded; B=code_mode_host_present_zero_tools_callback_not_applicable; C=app_server_initialized_thread_started_no_turn.
