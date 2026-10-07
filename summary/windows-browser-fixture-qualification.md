# Windows browser fixture qualification

Slice305 cross-compiled the service ingress fixture for Windows and ran both
fixture and Chrome in the same Windows loopback namespace. The browser
rendered the service page and completed same-origin fetch successfully. This
does not qualify production WFP/profile/clean-VM/Provider behavior.
