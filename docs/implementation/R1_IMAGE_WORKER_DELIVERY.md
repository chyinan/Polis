# R1 image input delivery

Updated: 2026-09-25. Status: local Worker-turn wiring implemented; real-provider vision qualification not run.

Validated PNG/JPEG uploads arrive as partial MissionInputs. New Task input manifests retain these images as immutable candidates. Directory and ZIP snapshots can contribute validated PNG/JPEG entries alongside supported text. Every image is selected from the frozen manifest/CAS snapshot and remains tied to the exact input/file SHA-256.

The bounded image profile admits at most four images per turn, 4 MiB per image and 8 MiB total image bytes. The context retains image bytes internally and includes only the display path/name, media type, size and digest in its textual trust-boundary section. Unsupported media, malformed images and over-limit images are rejected or recorded as exclusions. GIF, SVG, OCR and other image transformations are not admitted by this profile.

Before the Worker turn, Control regenerates the context from CAS and verifies it against the frozen Task manifest. It records image references with the append-only delivery receipt and projects them through Workbench. The Codex adapter adds each selected image as an app-server `type: image` input with a `data:image/...;base64,...` URL and `detail: auto`. Outgoing protocol evidence replaces image bytes with a redaction marker and payload digest.

Offline tests verify direct and archived image selection, delivery receipt integrity, exact bytes passed to the fake Worker turn options, data-URL construction against a scripted app-server, and protocol-log redaction. The locally installed Codex CLI 0.151 JSON schema defines `ImageUserInput` as an image URL; the product runtime manifest's 0.154 binary was not launched for this slice. The official [OpenAI Images and vision guide](https://developers.openai.com/api/docs/guides/images-vision) documents base64 data-URL image inputs for the underlying API. No real model turn or provider egress occurred; real model vision behavior remains unqualified.
