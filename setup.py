from setuptools import setup

setup(
  name='jetstore',
  author='ArtiSoft',
  version='1.0',
  python_requires='>=3.9',
  install_requires=['absl-py', 'apsw', 'antlr4-python3-runtime'],
  packages=['jets', 'jets.bridge', 'jets.compiler'],
  package_data = {
    'jets.bridge': ['jetrule_rete_test.db'],
    'jets.compiler': ['JetRule.g4', 'test_data/*', '*.interp', '*.tokens']
  },
  # include_package_data=True,
  license='ArtiSoft Inc.',
)
