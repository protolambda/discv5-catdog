# Discovery v5 CATDOG

This is a frankestein-eseque creation to help migrate Eth2 from disc v5.0 to v5.1.
Hacked together by @protolambda, based on discord discussion.

Have fun, this is not built for long-running usage, but should make migration more smooth.

Modifications to both discovery version copies:
- Strip out v4 files, no need to make it cat-dog-bird
- Watch for seen nodes, have the catdog copy it over to the other end
- Intercept revalidation, and make it try both v5.0 and v5.1 pings

And then there is the Catdog instance: same config etc., but two identities, two connections, and running both versions!
The common packages (ENR, enode, UDP connection, log, etc.) this uses the latest Geth as library (which just upgraded into v5.1).
For the actual discovery code of both versions, minimal copies are used instead.

## License

Most discovery code is a copy of that in go-ethereum. See COPYING file for details.
The rest is a temporary hack to help the discv5 protocol.
