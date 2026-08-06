# Interactive Resolution Example

This example declares an OpenAI environment contract without providing a
`.env` file. The required `OWL_INTERACTIVE_OPENAI_API_KEY` remains unresolved until
an interactive client supplies it.

From this directory:

```bash
owl snapshot --interactive
```

The CLI uses Charm Bubble Tea input for the interactive value and applies the answer
through Owl's normal resolver proposal path with `[prompt]` provenance.

To inspect the generated dotenv contract:

```bash
owl project spec --output -
```
