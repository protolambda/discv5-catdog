# Discovery v5 CATDOG

This is a frankestein-eseque creation to help migrate Eth2 from disc v5.0 to v5.1.
Hacked together by @protolambda, based on discord discussion.

Have fun, this is not built for long-running usage, but should make migration more smooth.

Modifications to both discovery version copies:
- Strip out v4 files, no need to make it cat-dog-bird
- Watch for seen nodes, have the catdog copy it over to the other end
- Intercept revalidation, and make it try both v5.0 and v5.1 pings
- Catdog instance: same config etc., but two identities, and running both versions!

## License

Most discovery code is a copy of that in go-ethereum. See COPYING file for details.
The rest is a temporary hack to help the discv5 protocol.
