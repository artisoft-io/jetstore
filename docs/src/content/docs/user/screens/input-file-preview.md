---
title: Input File Preview
description: The Input File Preview screen, the first lines of a file as it arrived in the installation's bucket, before it is loaded or mapped.
kind: reference
---

*Input File Preview* shows the first lines of a file as it arrived in the installation's bucket,
before JetStore loads or maps it: the header line, the delimiters, the quoting, exactly as the
vendor wrote them. Use it when a load or a mapping does not give what you expected, to check the
file before the configuration.

**Capability:** `jetstore_read`.

**Reached from** the [Load Files](/docs/user/screens/flows/load-files/) flow: select a file, then
*Preview File*. The screen's address carries the file's key, `filePreviewPath/<file key>`.

**In this version *Preview File* shows *Page not found* instead.** It puts the file key into the
address as it is, folders and all, and the screen's address takes the key as a single part, so no
file key with folders reaches the screen. Every file key has folders.

The screen is headed *Input File Preview*, and holds one table, captioned *Preview of* and the file
key. It has one column, **Line**, and a row for each line of the file, as text, up to its first 50
lines; a line is not split into columns. The table has no buttons.

The file is read from the installation's bucket each time the screen opens, so it shows the file as
it is now. A file that is compressed, or in a binary format such as Parquet, is shown as its raw
bytes, which are not readable as text.
