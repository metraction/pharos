#!/bin/bash

set -e

IMAGE_FILE=$1

TARGET_REGISTRY=pharos.secimo.net
REGISTRY_USER=pharos
ARCHITECTURES="amd64 arm64"

echo "infile: $IMAGE_FILE"
echo "target: $REGISTRY_USER@$TARGET_REGISTRY ($ARCHITECTURES)"

echo "--"
user=$(podman login --get-login $TARGET_REGISTRY)

if [[ "$user" != "$REGISTRY_USER" ]]; then
    echo "ERROR: podman login of $REGISTRY_USER for $TARGET_REPO required"
    
    exit 1
fi
if [ ! -f "$IMAGE_FILE" ]; then
  echo "ERROR: file $IMAGE_FILE not found"
  exit 1
fi

echo ""
while IFS= read -r IMAGE || [[ -n "$IMAGE" ]]; do
  # skip empty lines and comments
  if [[ "$IMAGE" == "# exit" ]]; then
    echo "exit command"
    exit 0
  fi
  if [[ -z "$IMAGE" || "$IMAGE" =~ ^# ]]; then
    continue
  fi
  # Extract image name and tag
  IMAGE_BASENAME="${IMAGE%%:*}"
  IMAGE_TAG="${IMAGE##*:}"
  TARGET_IMAGE="${TARGET_REGISTRY}/${IMAGE_BASENAME}:${IMAGE_TAG}"

  echo "-----< $IMAGE >-----"
  echo "1. pull and tag single-arch images"
  for ARCH in $ARCHITECTURES; do
    skopeo copy --override-os linux --override-arch=$ARCH docker://$IMAGE docker://$TARGET_IMAGE-$ARCH
  done

  echo "2. create manifest list"
  podman manifest create $TARGET_IMAGE
  
  echo "3. add the images for each architecture"
  for ARCH in $ARCHITECTURES; do
    podman manifest add $TARGET_IMAGE docker://$TARGET_IMAGE-$ARCH
  done

  echo "4. push manifest list to the target registry"
  podman manifest push --all $TARGET_IMAGE docker://$TARGET_IMAGE
  echo ""


done < "$IMAGE_FILE"

echo "dooine"
exit 1