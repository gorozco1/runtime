#!/bin/bash
# -*- mode: shell-script; indent-tabs-mode: nil; sh-basic-offset: 4; -*-
# ex: ts=8 sw=4 sts=4 et filetype=sh
#
# Automation script to create specs to build cc-runtime
# Default: Build is the one specified in file configure.ac
# located at the root of the repository.
set -x
AUTHOR=${AUTHOR:-$(git config user.name)}
AUTHOR_EMAIL=${AUTHOR_EMAIL:-$(git config user.email)}

VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))
hash_tag=$(git log --oneline --pretty="%H %d" --decorate --tags --no-walk | grep $VERSION| awk '{print $1}')
short_hashtag="${hash_tag:0:7}"
# If there is no tag matching $VERSION we'll get $VERSION as the reference
[ -z "$hash_tag" ] && hash_tag=$VERSION || :

OBS_PUSH=${OBS_PUSH:-false}
OBS_RUNTIME_REPO=${OBS_RUNTIME_REPO:-home:erick0zcr/cc-runtime}

GO_VERSION=${GO_VERSION:-"1.8.3"}

echo "Running: $0 $@"
echo "Update cc-runtime $VERSION: ${hash_tag:0:7}"

function changelog_update {
    d=$(date +"%a, %d %b %Y %H:%M:%S %z")
    git checkout debian.changelog
    cp debian.changelog debian.changelog-bk
    cat <<< "cc-runtime ($VERSION) stable; urgency=medium

  * Update cc-runtime $VERSION ${hash_tag:0:7}

 -- $AUTHOR <$AUTHOR_EMAIL>  $d
" > debian.changelog
    cat debian.changelog-bk >> debian.changelog
    rm debian.changelog-bk
}
changelog_update $VERSION

sed -e "s/@VERSION@/$VERSION/g;" -e "s/@GO_VERSION@/$GO_VERSION/g;" cc-runtime.spec-template > cc-runtime.spec
sed -e "s/@VERSION@/$VERSION/g;" -e "s/@HASH_TAG@/$short_hashtag/g;" cc-runtime.dsc-template > cc-runtime.dsc
sed -e "s/@VERSION@/$VERSION/g;" -e "s/@HASH_TAG@/$short_hashtag/g;" debian.rules-template > debian.rules
sed "s/@VERSION@/$VERSION/g;" _service-template > _service

# Update and package OBS
if [ "$OBS_PUSH" = true ]
then
    temp=$(basename $0)
    TMPDIR=$(mktemp -d -t ${temp}.XXXXXXXXXXX) || exit 1
    osc co "$OBS_RUNTIME_REPO" -o $TMPDIR
    mv cc-runtime.spec \
        cc-runtime.dsc \
        _service \
        debian.rules \
        $TMPDIR
    rm $TMPDIR/*.patch
    cp debian.changelog \
        debian.compat \
        debian.control \
        debian.postinst \
        debian.series \
        *.patch \
        $TMPDIR
    cd $TMPDIR

    if [ ! -e "go${GO_VERSION}.linux-amd64.tar.gz" ]; then
        rm go*.tar.gz
        curl -OkL https://storage.googleapis.com/golang/go$GO_VERSION.linux-amd64.tar.gz
    fi
    osc addremove
    osc commit -m "Update cc-runtime $VERSION: ${hash_tag:0:7}"
fi
