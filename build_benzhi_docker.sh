#!/bin/bash
set -e

IMAGE_NAME=${1:-my-project}
DOCKER_PLATFORM=${2:-linux/amd64}

docker build --platform "$DOCKER_PLATFORM" -f benzhi.Dockerfile -t "$IMAGE_NAME" .

echo ""
echo "Docker image '$IMAGE_NAME' built successfully!"
echo ""
echo "Next steps (for testing):"
echo "  * smoke test: docker run $IMAGE_NAME:latest"
echo "  * interactive shell: docker run -it $IMAGE_NAME:latest /bin/sh"
