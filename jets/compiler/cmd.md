# Useful commands for developing JetRule language
See docker_notes.md at root of project.
## Generating Antlr4 Lexers and Parsers

### Generating the python base visitor
Our main interest is to implement a python visitor for processing
the generated parse tree. The command to generate the python
parser is:
```
antlr4 -Dlanguage=Python3 JetRule.g4 -o py
```
The python code will be generated in the `py/` sub-directory.

#### Testing the the custom JetRule listener
```
python3 jetListener.py test.jr
```

### Generating the java parser for command line testing
To perform quick testing of an in-progress grammar, using the
java generated lexer parser allows for quick testing using the
`grun` alias:
```
antlr4  JetRule.g4  -o java
cd java
javac JetRule*.java
grun JetRule jetrule -tree
```
The java lexer and parser are generated in the `java/` directory.

#### Combining in fewer commands
```
 antlr4 JetRule.g4 -o java && cd java && javac JetRule*.java 
 grun JetRule jetrule -tree 
 cd ..
 ```

 ### Bazel Commands
 Rule the compiler with bazel
 ```
bazel run //jetstore-tools/jetrule-grammar:jetrule_compiler -- --in_file jetrule_main_test.jr --base_path jetstore-tools/jetrule-grammar  --rete_db jetrule_rete.db
```