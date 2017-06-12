# Sircles deployments

## Single process

The sircles backend can be built to also embed all the ui assets and provides them in the same http endpoint. In this way you can just provide the sircles configuration file and execute it (and scale it to N instances).

Obviously the config file should be the same for every instance. If you're willing to just do some testing you can also use a local sqlite3 database instead of postgres (in this case you can only have one instance)


## Split ui and backend

you can split the ui and the backend. You can expose the using a web server like apache httpd, nginx, caddy etc... Since the ui is a react app and we are using react router with browser history be aware of the "tricks" needed for handling correct fetching of the application assets (for example during application reload) since a reload in the browser under a specific url should always fetch the index.html available at the context root. For example for nginx a configuration should be:

```
  location / {
    root /path/to/the/sircles/ui/assets;
    try_files $uri /index.html;
```

in this way if the browser url is in a location like `https://your.sircles.installation/role/EkSTp4dx3chMQM43jmzfeb` the browser will ask for a file at `/role/EkSTp4dx3chMQM43jmzfeb` that doesn't exists and the `index.html` will be returned.

### Same ui and backend domains

If you're willing to serve the ui and the api from the same domain you can configure your web server to proxy requests starting from `/api/` to the backend. With nginx you can do something like:

```
  location /api {
    proxy_pass http://internal.api.host:8080;
  }
```

### Split ui and backend domains

If you want to serve the ui and the api from different domains you can:

* do just as above using split web servers configuration
* directly expose the api server (possibly exposing it only via https).

In addition you should also enable CORS in the api server under the web part in the config setting the `allowedOrigins`:

```
web:
  allowedOrigins:
    - "https://client-domain"
```

You can also specify `*` to accept every origin (for for info see the [CORS specification](https://www.w3.org/TR/cors/))

### ui config

The ui needs to know the api url. To set this you can put a file called `config.js` inside the ui root. The contents of the file are these:

```
const CONFIG = {
  apiBaseUrl: 'https://your.sircles.api/api',

  /* uncomment this when using openid connect authentication */
  //authType: 'oidc'
}

window.CONFIG = CONFIG
```

### High Availability

As explained above the sircles backend and frontend can be scaled to N instances.

The production database must be PostgreSQL (in future we'd like to enable support for CockroachDB) so it needs to be made high available. This could be done in many different ways. We develop and maintain an ooopen source cloud native PostgreSQL high availability solution called [**stolon**: https://github.com/sorintlab/stolon](https://github.com/sorintlab/stolon)
