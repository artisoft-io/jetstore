"""Domain model for the sample app, expressed with Pydantic.

The Pydantic models produce a JSON Schema that is used both to constrain
Ollama's structured output and to validate the model response client-side.
"""

from typing import Annotated, Literal, Union

from pydantic import BaseModel, Field, TypeAdapter

Status = Literal["todo", "in_progress", "done"]


class EmailTask(BaseModel):
    type: Literal["email"]
    id: str
    title: str
    status: Status
    recipient: str


class ReviewTask(BaseModel):
    type: Literal["review"]
    id: str
    title: str
    status: Status
    reviewer: str


class BatchJobTask(BaseModel):
    type: Literal["batch_job"]
    id: str
    title: str
    status: Status
    recordCount: int


Task = Annotated[
    Union[EmailTask, ReviewTask, BatchJobTask],
    Field(discriminator="type"),
]

# Validates/serializes a JSON array of Task objects.
TaskList = TypeAdapter(list[Task])
