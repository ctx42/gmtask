# ==============================================================================
# Creates image with user "enki" which has UID and GID set from arguments.
# ==============================================================================
ARG C42_BLD_IMG_BASE
FROM $C42_BLD_IMG_BASE AS project

# User ID to use when creating the "enki" user (required).
ARG C42_USR_UID

# Group ID to use when creating the "enki" user (required).
ARG C42_USR_GID

# Group ID used by docker (optional, default: 7000).
ARG C42_DKR_GID=7000

RUN groupadd -g $C42_DKR_GID docker && \
    groupadd -r -g $C42_USR_GID enki && \
    useradd -r -M -u $C42_USR_UID -g $C42_USR_GID -G docker enki

USER enki
