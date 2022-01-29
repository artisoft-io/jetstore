import sys
from typing import Dict, Sequence
from jetrule_context import JetRuleContext
import json
import io
import re
from absl import app
from absl import flags


class JetRuleValidator:

  def __init__(self, ctx: JetRuleContext):
    pass


FLAGS = flags.FLAGS

flags.DEFINE_string('jr', None, 'jetrule file.')

def readFile(fname: str, pat, fout: io.StringIO):
  with open(fname, 'rt', encoding='utf-8') as fjr:

    while True:
      jline = fjr.readline()
      if not jline:
        break

      m = pat.match(jline)
      if m:
          print('Match found: ', m.group(1))
          readFile(m.group(1), pat, fout)
      else:
        fout.write(jline)


def main(argv):
  # if FLAGS.debug:
  #   print('non-flag arguments:', argv)
  print('Reading file', FLAGS.jr, 'and importing included files recursively')
  pat = re.compile(r'import\s*"([a-zA-Z0-9_\/.-]*)"')
  
  # def output_file(out: TextIO, fin: TextIO, keep_license: bool) -> None:
  fout = io.StringIO()
  fname = FLAGS.jr
  readFile(fname, pat, fout)
  
  # Done reading content
  fout.seek(0)
  print('Read this:\n',fout.read())


if __name__ == '__main__':
  app.run(main)
