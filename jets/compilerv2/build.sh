#!/bin/bash
set -e

antlr='java -jar /usr/local/lib/antlr-4.13.2-complete.jar'

$antlr -Dlanguage=Go JetRule.g4 -o parser
