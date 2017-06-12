# General FAQ

## Is sircles Holacracy® compliant?

Short answer: No, Long answer: we developed sircles to fullfill our requirements on handling our internal organization. We use sircles to implement holacracy and wanted to make it as easy as possible. Internally we follow Holacracy but sometimes we branch some steps to other methodologies based on circles needs (like Kanban).

The official compliant holacracy app is [glassfrog](https://glassfrog.com)

## Does it implement governance and tactical meeting?

Not currently. We found easier for us to do meetings outside a tool and then just use sircles to commit the meetings outcomes.
Also based on user feedback in future we could add some meeting management to sircles but in a way that it'll leave the freedom to handle meeting in multiple ways without forcing users to use sircles for every meeting step.

## How does tension and proposal works?

In our workflow people registers their tensions and can assign them to a circle. When no circle is set in the tension only the user can see it, when a circle is assigned also the circle lead link can see it. In the tension description people can describe one or more proposals that will be discussed in the meetings. The proposal is just written, we choosed to not make it a form to design changes to a circle since we found that:

* sometimes a proposal can be fixed in other ways than just changing the organization structure
* when a change to a circle is the right solution, what to change should be decided during the meeting

## I noticed that I'm not forced to set elected roles election duration...

Yes you aren't forced. If you prefer to strictly follow Holacracy then just always set it. In future we could also add a "holacracy strict mode" option to force some behaviors.


# Technical FAQ

Please first take a look at the [sircles architecture](architecture.md)

## I don't like the ui

As explained in the [sircles architecture](architecture.md) the ui just uses the Sircles API and since they're evolving we just tried to make it the entry point for an initial good user experience.
Since we focused more on the backend many things will be added and change in the ui. Once stabilized we should really start writing all the tests (now totally missing :( ).

We're open for any suggestion and accept pull requests.

# Are the graphql API stable?

No. We are experimenting with them and many things could change (like improved pagination).

# How are upgrades handled?

We would like to keep the events (which are the real source of truth for all the other backend components) backward compatible and just add new events or upgrade existing events version when there's the need.

The "read database" instead could be drastically changed, but given the CQRS and event sources architecture it could be rebuild at every time starting from the events. That's the way we'll provided upgrades for the first releases until we'll settle with a stable readdb schema.
