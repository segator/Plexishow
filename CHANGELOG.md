# Changelog

## 1.0.0 (2026-05-18)


### Features

* add OCI Helm chart support with Gateway API, dynamic config serialization, and updated CI/CD publishing flow ([98f8835](https://github.com/segator/Plexishow/commit/98f8835f95cdf59366b8d6c246bdae1552f9e646))
* initial implementation of Plexishow IPTV proxy ([1babd79](https://github.com/segator/Plexishow/commit/1babd79274ba7fde421879f898b0cbe9238085bb))
* integrate OCI Helm chart publishing into CI pipeline and update versioning logic ([3dd2231](https://github.com/segator/Plexishow/commit/3dd2231b8cce63ed801471dc1bbee47a4b428325))
* multi-arch Docker builds via docker:build-push ([aa81038](https://github.com/segator/Plexishow/commit/aa81038d3b5a3c6c86848238fca77a8cb19071dc))


### Bug Fixes

* add explicit ARG TARGETARCH for buildx compatibility ([ce1496b](https://github.com/segator/Plexishow/commit/ce1496beac5764fc96b1055a6af270062972b1d4))
* align Docker tags with release strategy ([ed14b42](https://github.com/segator/Plexishow/commit/ed14b429ad14bb634d48e1cf01663266ea76e9ba))
* avoid redundant binary build and asset generation in CI publish step ([6e64e5f](https://github.com/segator/Plexishow/commit/6e64e5fe897258e138ad4d308226eba2ad4fc93f))
* change Helm push OCI registry URL to flat format to solve GITHUB_TOKEN scope permissions ([5641cb0](https://github.com/segator/Plexishow/commit/5641cb0e236344620c76f61bbfb5b37f12ee049e))
* conditionally install Intel GPU packages only on amd64 for multi-arch builds ([b21dbdb](https://github.com/segator/Plexishow/commit/b21dbdb7efee949577bdc56979d8c27a9bd695ce))
* include assets/ in Docker build context and group config docs by block ([12feb01](https://github.com/segator/Plexishow/commit/12feb0147645057f548d40c002a2ec102162a399))
* remove latest tag (not requested) ([aa55f26](https://github.com/segator/Plexishow/commit/aa55f2672ebe8abe212392bbddcdb6c881470f4b))
* remove redundant rebuild in docker:publish and push latest tag ([126a339](https://github.com/segator/Plexishow/commit/126a339803b2aa8d43fc91b82273e61b519f8150))
* rename Helm OCI package path to avoid UI name collision with Docker image and remove latest docker tag in docs ([c36bf84](https://github.com/segator/Plexishow/commit/c36bf8413041405863373fe985bdff3ae57be539))
