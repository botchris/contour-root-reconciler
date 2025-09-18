release:
	docker buildx build --no-cache --platform linux/amd64,linux/arm64,linux/386 -f ./Dockerfile -t botchrishub/contour-root-reconciler:latest --push .
