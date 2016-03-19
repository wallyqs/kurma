# Begin from Kurma stage4
FROM apcera/kurma-stage4

# Run the build.
COPY build.sh kernel.defconfig patches /tmp/
RUN /tmp/build.sh && rm /tmp/build.sh
