# Sircles - Enable the Evolutionary representation of your organization structure, roles and people.

## Features

* API
  * GraphQL API to build you own ui and tools
  * Time travelling queries: get the state at a precise point in time. See how your organization was at a specific date.

* Web UI
  * Time travel your organization
  * Organization chart
  * Manage circles and roles
  * Create tensions
  * Manage members
  * Basic search

## Sircles Specification 

In the next weeks (as of 13/Nov/2017) we'll publish our Sircles specification where we'll document the outcomes of our experiences implementing the Sorint.Lab organizational model. Sircles will then receive all the required changes to implement the Sircles specification.

Stay tuned!


## Architecture

The sircles core (backend) is written in Go and exposes a GraphQL API.

On top of it we provide a React based UI (but any kind of client can be built using the API).

## Documentation

[Documentation Index](doc/README.md)

## Quick start and examples

### Quick start using docker

To quickly see how sircles works just use the provided docker image.

```
docker run -p 80:8080 -it sorintlab/sirclesdemo
```

you can then login as user `admin` with password: `password`


This is just for quickly trying Sircles. For real production deployments see the [related doc](doc/deployments.md)

## Project Status

Sircles is under active development.

## Requirements

* PostgreSQL >= 9.5

## FAQ

See [here](doc/faq.md) for a list of faq. If you have additional questions please ask.

## Contributing to Sircles

sircles is an open source project under the Apache 2.0 license, and contributions are gladly welcomed!
To submit your changes please open a pull request.

## Contacts

* For bugs and feature requests file an [issue](https://github.com/sorintlab/sircles/issues/new)
* For general discussion about using and developing sircles, join the [sircles](https://groups.google.com/forum/#!forum/sircles) mailing list
* For real-time discussion, join us on [Gitter](https://gitter.im/sorintlab/sircles)
