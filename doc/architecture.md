# Sircles architecture

We were primarily focused on the Sircles core and created sircles also as a testbed to experiment with different emerging technologies like:

* GraphQL
* CQRS and event sourcing (to some extents)

## Sircles Core (backend)

The backend is the core of sircles, it exposes a GraphQL API so everyone can implement its own client or tools around it.

We are implementing a CQRS and event sourcing based architecture.

We split the command services from the read services. So the commands (that comes from GraphQL mutations or internal changes) are validated and trigger the creation of some events. The events are the source of truth of the aggregates states. These events are then processed by the read database that applies the related changes to its view.

For all details about the implementation we're going to detail them in different blog posts and future documentation.


### Read database architecture

The read database is built on top of sql but implemented with some additiona concepts:

* Immutable database. Every change is done as a new row. So the same entity is recorded as multiple rows and every row contains its own validity time range. This is used for time travelling the sircles organization.
* Graphs. Many concept are mapped to a graph (with vertex and edges). This concept pairs very well with the immutable database structure.


## User web interface

The web ui is a React application that (at least we think so) should provide a good user experience but is very young and could be greatly improved. It provides some interesting things like the organization chart.
