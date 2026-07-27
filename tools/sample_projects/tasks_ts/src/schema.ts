import { Type, type Static } from "@sinclair/typebox";

/**
 * Domain model for the sample app, expressed with TypeBox.
 *
 * TypeBox schemas ARE JSON Schema at runtime, so the same object is used for:
 *   1. Ollama structured output (passed as the `format` field), and
 *   2. Client-side validation of the model response.
 */

export const Status = Type.Union(
  [Type.Literal("todo"), Type.Literal("in_progress"), Type.Literal("done")],
  { $id: "Status" },
);

export const EmailTask = Type.Object(
  {
    type: Type.Literal("email"),
    id: Type.String(),
    title: Type.String(),
    status: Status,
    recipient: Type.String(),
  },
  { additionalProperties: false },
);

export const ReviewTask = Type.Object(
  {
    type: Type.Literal("review"),
    id: Type.String(),
    title: Type.String(),
    status: Status,
    reviewer: Type.String(),
  },
  { additionalProperties: false },
);

export const BatchJobTask = Type.Object(
  {
    type: Type.Literal("batch_job"),
    id: Type.String(),
    title: Type.String(),
    status: Status,
    recordCount: Type.Integer(),
  },
  { additionalProperties: false },
);

export const Task = Type.Union([EmailTask, ReviewTask, BatchJobTask]);

export const TaskArray = Type.Array(Task);

export type Status = Static<typeof Status>;
export type Task = Static<typeof Task>;
