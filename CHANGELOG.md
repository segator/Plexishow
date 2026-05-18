# Changelog

## 1.0.0 (2026-05-18)


### Features

* add Dagger Cloud cache sharing for CI and local dev ([4fbac57](https://github.com/segator/Plexishow/commit/4fbac572289329a08008221cb23e720961959c6d))
* add ffmpeg per-stream logging, improve streaming performance and buffering, and implement active stream monitoring ([2d2d282](https://github.com/segator/Plexishow/commit/2d2d282a37a562802085f80548ed6ec37e60dd6b))
* add FFmpeg script to generate and serve a placeholder video for zero-latency stream loading ([aa0e6bd](https://github.com/segator/Plexishow/commit/aa0e6bdcd01d75eaf97ba5fdedd491d3b73ba1e0))
* **ci:** add coverage gate and golangci-lint via Dagger ([e639d38](https://github.com/segator/Plexishow/commit/e639d38aab61b7527a15a1d14356551418d14652))
* **cli:** restore -token flag to override per-channel X-TCDN-token ([2a74146](https://github.com/segator/Plexishow/commit/2a74146a5cf931422b5cd10eef8984864fb6da07))
* dagger cache, test coverage, CI polish ([a370fc2](https://github.com/segator/Plexishow/commit/a370fc2fc9796ec85ff91f6bfc87aa9fca400217))
* **epg:** auto-discover EPG URL from #EXTM3U url-tvg attribute ([cfd9ac4](https://github.com/segator/Plexishow/commit/cfd9ac4b2b0ffdbb94b5703a071a5a98fa8fe330))
* implement advanced FFmpeg configuration and extend metrics with per-channel tracking ([1c921c0](https://github.com/segator/Plexishow/commit/1c921c06fafea395323a133ee0346917bff5900b))
* initial implementation of Plexishow IPTV proxy ([9d122f0](https://github.com/segator/Plexishow/commit/9d122f0dcdad16032f9afbf8eca4c63dc070eff9))
* log channel URLs on boot and prefix ffmpeg stderr per channel ([d9b4fac](https://github.com/segator/Plexishow/commit/d9b4fac5959ea5aaacb775d4af1a68100409a592))
* log internal stream URLs on boot instead of M3U source URLs ([ab7d905](https://github.com/segator/Plexishow/commit/ab7d90520f20ddbeb2ab0d4e7b0bb16499536caa))
* **m3u:** parse X-TCDN-token from stream_headers, fix ffmpeg args ([1405b79](https://github.com/segator/Plexishow/commit/1405b79c3d0ab09db1166e5011efe82b011bba9d))
* **mage:** add run target to build and execute locally ([dc3d9b2](https://github.com/segator/Plexishow/commit/dc3d9b2dabb7dc56dea0cbe5a1350dcf629cc74b))
* **mage:** forward arguments from mage run to the binary ([0d1d754](https://github.com/segator/Plexishow/commit/0d1d7541f0a29ef367c5c05827997f0d68903cef))
* optimize FFmpeg stream startup latency by reducing probe sizes, disabling buffers, and adding zero-latency flags ([b1c20a9](https://github.com/segator/Plexishow/commit/b1c20a911f14b105aaff514111c82eaa3df29be9))
* reduce default M3U refresh to 5m for token freshness ([b42e815](https://github.com/segator/Plexishow/commit/b42e815c61b161ac2e87932d2f6395a36b6a562a))
* replace placeholder video with 30-second broadcast animation and implement automatic client disconnection on timeout ([3b34242](https://github.com/segator/Plexishow/commit/3b3424207d3daa1832d88b46655635bc363a1589))
* **security:** add CodeQL + govulncheck to CI and weekly security scan ([adcad9f](https://github.com/segator/Plexishow/commit/adcad9fbb3a9fed08e8b0231d69768b12246e15f))
* single Dagger session + OTLP tracing ([8bd4885](https://github.com/segator/Plexishow/commit/8bd488507da96d3c10204e27212e00ae940f3a45))
* **streamer:** log ffmpeg command on each stream start ([335e3a4](https://github.com/segator/Plexishow/commit/335e3a4e934aeed99bb20121ff516e7e50c9d58a))
* track client IPs in stream sessions, update stats logging, and filter verbose CENC warnings ([8c0bd00](https://github.com/segator/Plexishow/commit/8c0bd00b9d81077aded63ba9b2df5aaf34839c1f))
* validate ffmpeg availability on startup, fail fast with clear error ([5b6281b](https://github.com/segator/Plexishow/commit/5b6281b4ae491714fa7d2f42a533975c77d783a9))


### Bug Fixes

* **ci:** correct ghcr.io registry URL in docker login ([d801559](https://github.com/segator/Plexishow/commit/d8015591c9d342f75bff7d6ab4ab044504ec334d))
* **ci:** custom OTel service name and ci.branch attribute ([7e46775](https://github.com/segator/Plexishow/commit/7e4677535c84740ac4f2dd6be38ffb3c4faed618))
* **ci:** restore missing SBOM, vulnscan and SARIF upload steps; docker:build now builds both images ([433da3f](https://github.com/segator/Plexishow/commit/433da3fa7c0fb1eed70fa5e781095c578e9b03a5))
* **ci:** use CodeQL build-mode:none for Go (no compilation needed) ([590a9ba](https://github.com/segator/Plexishow/commit/590a9ba3aa5f14ea726052b31131683440dccffa))
* **config:** make config.yaml optional — app starts without it ([25132ab](https://github.com/segator/Plexishow/commit/25132abcd79c044b262e7a9d22cd64104596a24c))
* **epg:** pass cfg by pointer so EPGURL mutation is visible in main ([9a5911f](https://github.com/segator/Plexishow/commit/9a5911fbbe875d58118ab625e1d3b07660764c15))
* fix GitHub Actions workflows ([5896bf2](https://github.com/segator/Plexishow/commit/5896bf221620e04e3081d629e9bd9d0db90bd12b))
* fix mage cover + add metrics tests ([31f658c](https://github.com/segator/Plexishow/commit/31f658c12016da24d9c3ff92061348e6bb139023))
* **lint:** address gosec G115 integer overflow warning in channelColor ([47d2081](https://github.com/segator/Plexishow/commit/47d208124e05dac762de031bc68b926ab87e1f10))
* **mage:** use mg.Deps(Bin.Build) instead of mg.F for namespace methods ([ff4332d](https://github.com/segator/Plexishow/commit/ff4332dda891b8182bad6dc4258650ac28d6e526))
* move nolint:gosec to correct line for gosec G118 ([a7e61cc](https://github.com/segator/Plexishow/commit/a7e61cc05666eac517df1f791c73224d850c21c7))
* pass GHCR auth to Dagger engine for registry cache ([19cde4b](https://github.com/segator/Plexishow/commit/19cde4b23367430816182007e829eae6c36d21bd))
* resolve golangci-lint errors and optimize Dagger caching ([73b51dc](https://github.com/segator/Plexishow/commit/73b51dccdeb55aa3028280574a78831ecda96504))
* restrict log file permissions, optimize slice allocation, and add safety checks for byte metrics ([758a579](https://github.com/segator/Plexishow/commit/758a57953a91151c8c6106e6656d5852d68b7da9))
* **streamer:** concatenate all headers into single -headers flag ([78cdb26](https://github.com/segator/Plexishow/commit/78cdb2632123d5c439b15143c3ec258f64e3219f))
* **streamer:** pass only the key to -cenc_decryption_key (not keyId:key) ([6840bce](https://github.com/segator/Plexishow/commit/6840bce269fd29f392007a8a78ef995f253e92ed))
