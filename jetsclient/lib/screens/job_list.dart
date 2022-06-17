import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/data_table.dart';

const defaultPadding = 16.0;
final List<String> menuEntries = <String>[
  'Input Files',
  'Mapping Configurations',
  'Process Configurations',
  'Data Pipelines'
];
final List<VoidCallback> menuActions = <VoidCallback>[
  () {},
  () {},
  () {},
  () {}
];

class JobListScreen extends StatefulWidget {
  const JobListScreen({super.key});

  @override
  State<JobListScreen> createState() => _JobListScreenState();
}

class _JobListScreenState extends State<JobListScreen> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: appBar('JetStore Workspace', context),
      body: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(children: [
              const SizedBox(height: defaultPadding),
              Expanded(
                child: Image.asset('assets/logo.png'),
              ),
              const SizedBox(height: defaultPadding),
              Expanded(
                  flex: 8,
                  child: ListView.separated(
                    padding: const EdgeInsets.all(defaultPadding),
                    itemCount: menuEntries.length,
                    itemBuilder: (BuildContext context, int index) {
                      return TextButton(
                        onPressed: menuActions[index],
                        child: Center(child: Text(menuEntries[index])),
                      );
                    },
                    separatorBuilder: (BuildContext context, int index) =>
                        const Divider(),
                  ))
            ]),
          ),
          Expanded(
            flex: 5,
            child:
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const SizedBox(height: 2 * defaultPadding),
              Text(
                'Data Pipelines',
                style: Theme.of(context).textTheme.headline4,
              ),
              const SizedBox(height: defaultPadding),
              const MyDataTableSampleWidget(),
              const SizedBox(height: defaultPadding),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                TextButton(
                  onPressed: (() => _showDialog('Comming Soon!')),
                  child: const Text("New Pipeline",
                  style: TextStyle(fontWeight: FontWeight.w800)),
                )
              ])
            ]),
            // child: Center(
            //   child: Text('Welcome',
            //       style: Theme.of(context).textTheme.headline2),
            // ),
          ),
        ],
      ),
    );
  }

  void _showDialog(String message) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(message),
        actions: [
          TextButton(
            child: const Text('OK'),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ],
      ),
    );
  }
}
