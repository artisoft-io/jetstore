---
title: Tear down a stack
description: Lift the deletion guards, destroy a JetStore stack, and remove what survives it.
kind: how-to
sidebar:
  order: 9
---

`cdk destroy` will not complete while the guards are in place. In order: disable stack
`TerminationProtection`, disable Aurora `DeletionProtection`, then destroy. Two things survive
regardless and must be removed by hand if you mean to: the **infer persistent volume**
(`RemovalPolicy_RETAIN`, named by the `PersistentVolumeId` output) and an **imported bucket**
(`JETS_BUCKET_NAME`). A stack-created bucket is emptied and deleted with the stack.
