#! /bin/bash

IMAGE_NAME=robocar-simulator
TAG=$(git describe)
FULL_IMAGE_NAME=docker.io/cyrilix/${IMAGE_NAME}:${TAG}
BINARY=rc-simulator

GOTAGS="-tags netgo"

image_build(){
  local platform=$1


  GOOS=$(echo $platform | cut -f1 -d/) && \
  GOARCH=$(echo $platform | cut -f2 -d/) && \
  GOARM=$(echo $platform | cut -f3 -d/ | sed "s/v//" )
  VARIANT="--variant $(echo $platform | cut -f3 -d/  )"
  if [[ -z "$GOARM" ]] ;
  then
  VARIANT=""
  fi

  local binary_suffix="$GOARCH$(echo $platform | cut -f3 -d/ )"

  local containerName="$IMAGE_NAME-$GOARCH$GOARM"


  printf "\n\nBuild go binary %s\n\n" "${BINARY}.${binary_suffix}"
  mkdir -p build
  CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} GOARM=${GOARM} go build -mod vendor -a ${GOTAGS} -o "build/${BINARY}.${binary_suffix}" ./cmd/${BINARY}/

  buildah --os "$GOOS" --arch "$GOARCH" $VARIANT  --name "$containerName" from gcr.io/distroless/static
  buildah config --user 1234 "$containerName"
  buildah copy "$containerName" "build/${BINARY}.${binary_suffix}" /go/bin/$BINARY
  buildah config --entrypoint '["/go/bin/'$BINARY'"]' "${containerName}"

  buildah commit --rm --manifest $IMAGE_NAME "${containerName}" "${containerName}"
}

buildah rmi localhost/$IMAGE_NAME
buildah manifest rm localhost/${IMAGE_NAME}

image_build linux/amd64
image_build linux/arm64
image_build linux/arm/v7


# push image
printf "\n\nPush manifest to %s\n\n" ${FULL_IMAGE_NAME}
buildah manifest push --rm -f v2s2 "localhost/$IMAGE_NAME" "docker://$FULL_IMAGE_NAME" --all