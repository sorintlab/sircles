# Sircles architecture

We were primarily focused on the Sircles core and created sircles also as a testbed to experiment with different emerging technologies like:

* GraphQL
* CQRS and event sourcing (to some extents)

## Sircles Core (backend)

The backend is the core of sircles, it exposes a GraphQL API so everyone can implement its own client or tools around it.

We are implementing a simple (and probably not really compliant with some CQRS best practices) CQRS and event sourcing based architecture.

We split the command services from the read services. So the commands (that comes from GraphQL mutations or internal changes) are validated and trigger the creation of some events. The events are the source of truth of the aggregates states. These events are then processed by the read database that applies the related changes to its view.

We used a simpler (and less scalable since we don't need to scale so much) model:

* The eventstore and the read database lives on the same database (now a sql like postgres, sqlite). In this way the flow that starts from a mutation to the generation of events and the application of the events to the read database are done inside the same (serializable) transaction. In this way we simplified different things:

* The ui, after a successful mutation will update and found the updated data (in the react ui we could also do, thanks to the react apollo framework, optimistic updates and polling on some queries but this is left for the future if really needed)
* We catch at runtime if the generated events causes problems on the read view. Of course this should (and is) tested inside integration tests but for the moment is a good life saver to avoid possible corruptions that will require rewriting the events and reappling them or rolling back some changes.

* In addition the command part uses, as the aggregate state snapshot, the same read database (in future we could split them but now, to speed up development and testing, we used the same).


### Read database architecture

The read database is built on top of sql but implemented with some additiona concepts:

* Immutable database. Every change is done as a new row. So the same entity is recorded as multiple rows and every row contains its own validity time range. This is used for time travelling the sircles organization.
* Graphs. Many concept are mapped to a graph (with vertex and edges). This concept pairs very well with the immutable database structure.


## User web interface

The web ui is a React application that (at least we think so) should provide a good user experience but is very young and could be greatly improved. It provides some interesting things like the organization chart.
