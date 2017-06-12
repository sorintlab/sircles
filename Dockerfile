FROM fedora:latest

# looks like sqlite3 is already available (at least on fedora 25 image)
# RUN dnf install sqlite-libs

ADD bin/sircles-dockerdemo /sircles
ADD examples/dockerdemo/config.yaml /config.yaml

EXPOSE 8080

ENTRYPOINT ["/sircles"]
CMD ["serve", "-c", "/config.yaml"]
