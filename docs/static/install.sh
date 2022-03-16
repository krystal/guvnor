#!/bin/bash

# Guvnor Install Script
#
# This is a VERY simple script for installing Guvnor on a Linux host. Many assumptions are made
# and if you need something more complicated you might to do something different.
#
# Usage:
#
#      curl https://guvnor.k.io/instal.sh | sudo bash
#

set -e

get-latest-version() {
    if ! command -v curl &> /dev/null
    then
        echo "curl could not be found. Install curl before continuing." > /dev/stderr
        return 1
    fi

    if ! command -v jq &> /dev/null
    then
        echo "jq could not be found. Install jq before continuing." > /dev/stderr
        return 1
    fi

    local response=`curl -s https://api.github.com/repos/krystal/guvnor/releases/latest`

    local error=`echo $response | jq -r '.message'`
    if [[ "$error" == *"rate limit exceeded"* ]]; then
        echo "GitHub API rate limit exceeded. Try again later." > /dev/stderr
        return 1
    fi

    local latest_version=`echo $response | jq -r '.tag_name'`
    if [ "$latest_version" == "" ] || [ "$latest_version" == "null" ]; then
        echo "Could not get latest version of Guvnor from GitHub. Make sure you" > /dev/stderr
        echo "are connected to the internet and GitHub is available." > /dev/stderr
        return 1
    fi

    echo $latest_version | sed -e "s/^v//"
}

latest_version=`get-latest-version`

echo -e "\e[35mGuvnor Installer\e[0m"
echo -e "\e[33mLatest version is $latest_version\e[0m"

release_url="https://github.com/krystal/guvnor/releases/download/v${latest_version}/guvnor_${latest_version}_Linux_x86_64.tar.gz"
tmp_tgz_path="/tmp/guvnor-dist.tgz"
tmp_extract_root="/tmp/guvnor"
install_path="/usr/local/bin/guvnor"

echo "Downloading from $release_url..."
rm -f $tmp_tgz_path
curl -L -s -q -o $tmp_tgz_path $release_url

echo "Extracting..."
rm -rf $tmp_extract_root
mkdir -p $tmp_extract_root
tar zxf $tmp_tgz_path -C $tmp_extract_root

echo "Moving..."
cp $tmp_extract_root/guvnor $install_path

#echo "Initializing..."
#guvnor init

echo -e "\e[32mGuvnor installed successfully at $install_path\e[0m"
