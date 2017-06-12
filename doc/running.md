# Running Sircles

after building you can find the sircles executable under `bin/sircles`

It's a single executable providing multiple subcommands.
The primary command is `serve` that will start the api server. It'll also provide (if not disabled at build time) the sircles web ui.

## Basic setup

## Configuring a database

Sircle stores its data in a SQL database. It supports PostgreSQL and sqlite3. You should use sqlite3 only for testing.

### PostgreSQL db setup

You should setup a PostgreSQL instance and create an empty database. Let suppose we'll call it `sircles`.

### Basic configuration file

Once created you have to define the connection string to the db in the sircles configuration file.

Supposing the PostgreSQL instance is listening on `localhost:5432` and the sircles api server will listen on `localhost:8080` a minimal basic configuration file can be:

``` yaml
web:
  http: 'localhost:8080'

db:
  type: 'postgres'
  connString: 'postgres://user:password@localhost/sircles?sslmode=disable'

tokenSigning:
  method: hmac
  key: supersecretsigningkey

authentication:
  type: local

```

save it as `config.yaml`


## Run the sircles api server

``` bash
bin/sircles serve -c config.yaml
```

At the first start it'll create the required database objects.

You can now access the sircles ui from you browser at 'http://localhost:8080'
