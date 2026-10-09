"""The declared scope, one module per token.

**Importing this package is the declaration.** `scope.declared_scope()` imports
it and then walks the registry the classes below put themselves into; there is
no list of tokens anywhere, here or elsewhere, and a class that is not imported
is not declared. That is the point: a token can only join the scope by someone
writing the class that implements it, and a class nobody imported fails loudly
the first time a pipeline names its token, rather than quietly.

**Eight tokens over three kinds, where the charter names six over one.** The
assessment's §4 and the Phase 9 charter both list the scope as *the `generator`
input channel, the site operator, `map_record`, `filter`, `partition_writer`,
`merge_files`* — which mixes an input-channel type, a pipe type and three
transformation tokens, and is short of what the phase's own pipeline needs by
two. `fan_out` is the pipe every transformation must sit inside, and `memory`
is the input-channel type by which the twelve partition writers read the site
operator's output channels. Neither is a widening of the subset: without them
the `.pc.json` P9-T18 authors cannot be written at all. Recorded as P9-I26 so
that the sweep rules on the wording rather than on an agent's reading.

**The site operator is not here**, and cannot be: its token is a deployment's
own, and a token named in this package would be a deployment named in this
package. It reaches the scope through `cpipes_node.site.Registry` instead,
which is where the Go builder's `default:` branch sends it.
"""

from __future__ import annotations

from . import (  # noqa: F401  (the import *is* the declaration)
    channels,
    pipes,
    transformations,
)
