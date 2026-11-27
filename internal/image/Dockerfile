# The adf desktop image: Debian with Xvfb, openbox, tint2, xdotool, scrot, Firefox ESR and
# fonts, provisioned by the same scripts adf uses when it builds the template locally.
# CI builds it for linux/amd64 and linux/arm64 and attaches it to a GitHub Release, so adf can
# create the desktop template without installing packages on the user's machine.
FROM debian:trixie-slim
COPY desktop-setup.sh /tmp/adf-setup.sh
COPY desktop-start.sh /usr/local/bin/desktop-start
RUN sh /tmp/adf-setup.sh && rm /tmp/adf-setup.sh && chmod 755 /usr/local/bin/desktop-start
ARG ADF_IMAGE_VERSION
RUN test -n "$ADF_IMAGE_VERSION" && echo "$ADF_IMAGE_VERSION" > /etc/adf-image-version
