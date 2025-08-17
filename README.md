# Townsquares relay

### Townsquares - Decentralised community focused social networks

This repository is home to the WIP Townsquares server/relay. The focus of this repository
is to built a decentralised, mesh-based relay system for hyper-localised communities.

#### Technical Goals

- ~~Create basic implementation of the relay server~~ ✅
- ~~Ensure the system is resiliant and propogates events relay-to-relay~~ ✅
- Ensure events are de-duped across the network when shared relay-to-relay
- Ensure users can use existing keys, ideally with existing 'signer' applications
- Ensure mesh 'bridges' don't propogate relay events to no-direct relays (avoid event leakage)
- ~~Create a dockerised implementation of the relay~~ ✅
- Explore other deployment options (desktop application, flatpak etc)


#### Documentation Goals

- Create basic README.md file for instructions on how to get up and running
- Create a Mermaid graph showcasing how clients and relays connect to each other
- Create a guide on how to set-up and deploy a Townsquares relay