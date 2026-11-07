# R0.3A-T4 normalized native differential

Historical protocol files contain no monotonic timestamps; normalized traces preserve wall/audit deltas and mark monotonic deltas as 
ot_recorded.

## First post-turn provider event

- r02-medium-1 [known_good]: `item/started`; first output=`2026-09-08T07:07:14.2024209+00:00`; tools=7; disconnects=0; terminal=``.
- r02-medium-2 [known_good]: `item/started`; first output=`2026-09-08T07:08:14.4809910+00:00`; tools=7; disconnects=0; terminal=`completed`.
- r02h3-high [known_good_auxiliary]: `item/started`; first output=`2026-09-08T11:29:14.2180099+00:00`; tools=4; disconnects=0; terminal=`completed`.
- r03a-initial-backend [failing]: `item/started`; first output=``; tools=11; disconnects=6; terminal=``.
- r03a-t2-backend [failing]: `item/started`; first output=``; tools=11; disconnects=3; terminal=``.
- r03a-t3-backend [failing]: `item/started`; first output=``; tools=11; disconnects=5; terminal=``.

## Difference matrix

- **CodexVersion** 鈥?`confirmed_same`; known-good=[0.151.0]; failing=[0.151.0].
- **BinarySHA256** 鈥?`confirmed_same`; known-good=[9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a]; failing=[9739cbc928b9c573be83256acd46668f5dd4f119d2d09e05246895ca2aaf0c9a].
- **CodeModeHostSHA256** 鈥?`not_recorded`; known-good=[a9adcea47799d8caaec5fbf073966fef2869f754bc7b463e30783880dfb12913]; failing=[].
- **SchemaDigest** 鈥?`confirmed_different`; known-good=[6032863e8a892b00eb34433d5fefb057066a7b700cf5f6ce06b1321e382b7455]; failing=[29781abec755d7acb618d76a40a82010fedd031af7f8165d5eee588665f5a1d4].
- **NativeProtocol** 鈥?`confirmed_same`; known-good=[passed_without_inference]; failing=[passed_without_inference].
- **Model** 鈥?`confirmed_same`; known-good=[gpt-5.6-luna]; failing=[gpt-5.6-luna].
- **Effort** 鈥?`confirmed_same`; known-good=[medium]; failing=[medium].
- **Cwd** 鈥?`confirmed_same`; known-good=[/work]; failing=[/work].
- **Sandbox** 鈥?`confirmed_same`; known-good=[read-only]; failing=[read-only].
- **ToolSchemaDigest** 鈥?`confirmed_different`; known-good=[d0886c0e93a53efd424ad48f7f02f7f9ae76ad46ff9a46a6aff316ac4b87cbd9]; failing=[b36063bf6b136b8116a1b744042e5f0228f46d559ba711e080e82b271e59645a].
- **ToolCount** 鈥?`confirmed_different`; known-good=[7]; failing=[11].
- **DeveloperInstructionBytes** 鈥?`confirmed_different`; known-good=[286]; failing=[185].
- **PromptInputBytes** 鈥?`confirmed_different`; known-good=[1137, 2168]; failing=[1110].
- **ThreadStartRequestBytes** 鈥?`confirmed_different`; known-good=[2975]; failing=[3717].
- **TurnStartRequestBytes** 鈥?`confirmed_different`; known-good=[1365, 2467]; failing=[1276].
- **ProxyEnvironment** 鈥?`not_recorded`; known-good=[]; failing=[].
- **HistoryContext** 鈥?`confirmed_same`; known-good=[not_recorded]; failing=[not_recorded].
- **CallbackReadiness** 鈥?`not_recorded`; known-good=[]; failing=[].
