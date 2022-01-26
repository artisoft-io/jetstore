# Useful commands for developing JetRule language
## Generating Antlr4 Lexers and Parsers

### Generating the python base visitor
Our main interest is to implement a python visitor for processing
the generated parse tree. The command to generate the python
parser is:
```
antlr4 -Dlanguage=Python3 JetRule.g4 -visitor -o py
```
The python code will be generated in the `py/` sub-directory.

#### Testing the the custom JetRule visitor
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
