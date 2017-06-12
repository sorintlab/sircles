# Building Sircles

to build sircles just run make inside the project root.

```
make
```

By default the `sircles` binary will also embed all the default sircles ui assets. So make will also build the sircles ui (calling `npm install` to fetch node_modules since we are not vendoring them).

Under `bin/` you'll find the `sircles` binary.

To disable this set the `NOWEBBUNDLE` environment variable:

```
NOWEBBUNDLE=1 make
```

## building the ui

to build the ui just issue:

```
make dist-web
```

## bulding the docker demo image

```
make dockerdemo
```

a docker image tagged as `sirclesdemo` will be built

you can run it as:

```
docker run -p 80:8080 -it sirclesdemo
```

you can then login as user `admin` with password: `password`
