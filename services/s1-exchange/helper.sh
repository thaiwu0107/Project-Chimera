#!/bin/bash
dirname=$PWD
SERVICE_NAME="${dirname%"${dirname##*[!/]}"}" 	# extglob-free multi-trailing-/ trim
PROHECT_NAME="${SERVICE_NAME##*/}"
SERVICE_NAME="demeter-${SERVICE_NAME##*/}"      # remove everything before the last /
IMAGE_TAG=`git rev-parse --short=6 HEAD`        # retrive tag to specified git tag
IMAGE_PREFIX="reg.paradise-soft.com.tw:5000/"   # this vary with different project name
IMAGE_NAME="$IMAGE_PREFIX$SERVICE_NAME:$IMAGE_TAG"

HELP_DOC="
    >$ ./helper.sh [param]
    (
      param:
        <build>     : build docker image with name 'project_name/folder_name:git_commit_tag'.
        <push>      : push docker images 'project_name/folder_name:git_commit_tag' to registry.
        <retrive>   : 執行專案映像檔，並把檔案匯出後關閉容器
        <remove>    : 刪除此專案的映像檔 
        <echo>      : 顯示此專案映像檔名稱
    )
"

function docker_build() {
    docker build --build-arg gitTag=$IMAGE_TAG -t $IMAGE_NAME .
    if [ $? != 0 ];then
        echo "fail to build..."
        exit 1
    fi
    echo 'build success'
}


function docker_push() {
    docker push $IMAGE_NAME
    if [ $? != 0 ];then
        echo "fail to push..."
        exit 1
    fi
    echo 'push success'
}


function docker_remove() {
    docker rmi $IMAGE_NAME
    if [ $? != 0 ];then
        echo "fail to push..."
        exit 1
    fi
    echo 'push success'
}


function retrive_binary() {
    # 1.processing
    docker run -it --rm --name=$PROHECT_NAME --entrypoint=/bin/bash -d $IMAGE_NAME
    # 2.wait a while (1s)
    sleep 1
    # 3.retrive binary from container
    docker cp $PROHECT_NAME:/app .
    # 4.stop processing
    docker stop $PROHECT_NAME
}


function echo_service() {
    echo $IMAGE_NAME
}


function help() {
cat << HELP 
    $HELP_DOC 
HELP
}



if [ "$1" == "build" ]; then
    docker_build
elif 
    [ "$1" == "push" ]; then
    docker_push
elif 
    [ "$1" == "remove" ]; then
    docker_remove
elif 
    [ "$1" == "retrive" ]; then
    retrive_binary
elif 
    [ "$1" == "echo" ]; then
    echo_service
else
    help
fi