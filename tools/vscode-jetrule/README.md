# JetRule Language Support

Syntax highlighting for the JetStore **JetRule** DSL (`.jr` files).

## Features

- Line comments (`#`)
- Strings with escape sequences
- Rule headers (`[RuleName, prop=val ...]:`) with rule-name and property highlighting
- Statement keywords: `class`, `lookup_table`, `triple`, `rule_sequence`, `main`, `jetstore_config`
- Declarators: `resource`, `volatile_resource`, `@JetCompilerDirective`
- Config/section properties: `$base_classes`, `$data_properties`, `$columns`, `$key`, `$main_rule_sets`, etc.
- Data types: `int`, `uint`, `long`, `ulong`, `double`, `text`, `date`, `datetime`, `bool`, `resource`, `array of`
- Constants: `true`, `false`, `null`
- Built-in functions: `create_uuid_resource`, `create_entity`, `toText`
- Rule variables (`?var`)
- Namespaced identifiers (`rdf:type`, `am:AnalysisRoot`)
- Operators: `== != <= >= < > + - * / = -> and or not r?`

## Development

1. Open this folder in VS Code.
2. Press `F5` to launch the Extension Development Host.
3. Open any `.jr` file to see highlighting.

## Packaging

```bash
npm install -g @vscode/vsce
vsce package
code --install-extension vscode-jetrule-0.1.0.vsix
```

The grammar is derived from `jets/compilerv2/compiler/JetRule.g4`.
