/// <reference types="node" />
import { readFileSync } from "fs";
import { Value } from "@sinclair/typebox/value";
import { TaskArray, type Task } from "./schema.js";

const OLLAMA_HOST = process.env.OLLAMA_HOST ?? "http://localhost:11434";
const OLLAMA_MODEL = process.env.OLLAMA_MODEL ?? "gemma4:latest";
const DEFAULT_PROMPT_FILE =
  "/home/michel/projects/repos/workspaces/jets_ws/data/patient_clinical_summary/initial_test_prompt.md";

interface Prompt {
  system: string;
  user: string;
}

/**
 * Parse a prompt file that contains a `System:` section followed by a `User:`
 * section, returning the two message bodies.
 */
function parsePrompt(text: string): Prompt {
  const match = /^\s*System:\s*([\s\S]*?)\n\s*User:\s*([\s\S]*)$/.exec(text);
  if (!match) {
    throw new Error("Prompt file must contain a 'System:' and a 'User:' section.");
  }
  return { system: match[1].trim(), user: match[2].trim() };
}

interface OllamaChatResponse {
  message?: { content?: string };
  error?: string;
}

async function callOllama(prompt: Prompt): Promise<string> {

  const payload = {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      model: OLLAMA_MODEL,
      stream: false,
      // Ask Ollama to constrain generation to our TypeBox schema (JSON Schema).
      format: TaskArray,
      options: { temperature: 0 },
      messages: [
        { role: "system", content: prompt.system },
        { role: "user", content: prompt.user },
      ],
    }),
  };
  // Print the payload for debugging purposes
  console.log("Payload sent to Ollama:\n", JSON.stringify(JSON.parse(payload.body), null, 2));

  const res = await fetch(`${OLLAMA_HOST}/api/chat`, payload);

  if (!res.ok) {
    throw new Error(`Ollama request failed: ${res.status} ${res.statusText}`);
  }

  const data = (await res.json()) as OllamaChatResponse;
  if (data.error) {
    throw new Error(`Ollama error: ${data.error}`);
  }
  return data.message?.content ?? "";
}

async function main(): Promise<void> {
  const promptFile = process.argv[2] ?? DEFAULT_PROMPT_FILE;
  const prompt = parsePrompt(readFileSync(promptFile, "utf8"));

  console.log(`Model      : ${OLLAMA_MODEL}`);
  console.log(`Ollama host: ${OLLAMA_HOST}`);
  console.log(`Prompt file: ${promptFile}\n`);

  const content = await callOllama(prompt);

  let parsed: unknown;
  try {
    parsed = JSON.parse(content);
  } catch {
    console.error("Model did not return valid JSON:\n");
    console.error(content);
    process.exitCode = 1;
    return;
  }

  if (!Value.Check(TaskArray, parsed)) {
    console.error("Model output failed schema validation:\n");
    for (const err of Value.Errors(TaskArray, parsed)) {
      console.error(`  ${err.path || "/"}: ${err.message}`);
    }
    console.error("\nRaw output:\n" + JSON.stringify(parsed, null, 2));
    process.exitCode = 1;
    return;
  }

  const tasks: Task[] = parsed;
  console.log(`Validated ${tasks.length} task(s):\n`);
  console.log(JSON.stringify(tasks, null, 2));
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : err);
  process.exitCode = 1;
});
