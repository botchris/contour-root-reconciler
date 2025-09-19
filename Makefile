release:
	docker buildx build --no-cache --platform linux/amd64,linux/arm64,linux/386 -f ./Dockerfile -t botchrishub/contour-root-reconciler:latest --push .

helm:
	helm package charts/contour-root-reconciler --destination docs/ --version 0.0.2
	helm repo index docs/ --url https://botchrishub.github.io/contour-root-reconciler/
