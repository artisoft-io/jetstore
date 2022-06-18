import 'package:flutter/material.dart';
import 'package:jetsclient/screens/job_list.dart';

class MyDataTableSampleWidget extends StatefulWidget {
  const MyDataTableSampleWidget({super.key});

  @override
  State<MyDataTableSampleWidget> createState() => _MyStatefulWidgetState();
}

class _MyStatefulWidgetState extends State<MyDataTableSampleWidget> {
  final ScrollController _verticalController = ScrollController();
  final ScrollController _horizontalController = ScrollController();

  @override
  Widget build(BuildContext context) {
    return _buildWithoutLayoutBuilder(context);
  }

  Widget _buildWithoutLayoutBuilder(BuildContext context) {
    return Scrollbar(
      thumbVisibility: true,
      trackVisibility: true,
      controller: _verticalController,
      child: SingleChildScrollView(
          scrollDirection: Axis.vertical,
          controller: _verticalController,
          child: Scrollbar(
            thumbVisibility: true,
            trackVisibility: true,
            controller: _horizontalController,
            child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                controller: _horizontalController,
                padding: const EdgeInsets.all(defaultPadding),
                child: _buildJetsDataTable(context)),
          )),
    );
  }

  // Widget _buildLayoutBuilder(BuildContext context) {
  //   return LayoutBuilder(
  //       builder: (BuildContext context, BoxConstraints viewportConstraints) {
  //     return Scrollbar(
  //       thumbVisibility: true,
  //       trackVisibility: true,
  //       controller: _verticalController,
  //       child: SingleChildScrollView(
  //           scrollDirection: Axis.vertical,
  //           controller: _verticalController,
  //           child: ConstrainedBox(
  //               constraints: BoxConstraints(
  //                 minHeight: viewportConstraints.maxHeight,
  //                 // maxHeight: 1800,
  //               ),
  //               child: Scrollbar(
  //                 thumbVisibility: true,
  //                 trackVisibility: true,
  //                 controller: _horizontalController,
  //                 child: SingleChildScrollView(
  //                     scrollDirection: Axis.horizontal,
  //                     controller: _horizontalController,
  //                     padding: const EdgeInsets.all(defaultPadding),
  //                     child: ConstrainedBox(
  //                       constraints: BoxConstraints(
  //                         minWidth: viewportConstraints.maxWidth,
  //                         // maxWidth: 2400,
  //                       ),
  //                       child: _buildJetsDataTable(context),
  //                     )),
  //               ))),
  //     );
  //   });
  // }

  Widget _buildJetsDataTable(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ElevatedButton(
          style: ElevatedButton.styleFrom(
            // Foreground color
            onPrimary: Theme.of(context).colorScheme.onSecondaryContainer,
            // Background color
            primary: Theme.of(context).colorScheme.secondaryContainer,
          ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
          onPressed: () => _showDialog('Coming Soon!'),
          child: const Text('New Pipeline'),
        ),
        _buildDataTable(context),
      ],
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

  Widget _buildDataTable(BuildContext context) {
    return DataTable(
      columns: <DataColumn>[
        DataColumn(
          label: Expanded(
            child: Text(
              'Job',
              style: Theme.of(context).textTheme.headline6,
            ),
          ),
        ),
        DataColumn(
          label: Expanded(
            child: Text(
              'Client',
              style: Theme.of(context).textTheme.headline6,
            ),
          ),
        ),
        DataColumn(
          label: Expanded(
            child: Text(
              'Process',
              style: Theme.of(context).textTheme.headline6,
            ),
          ),
        ),
        DataColumn(
          label: Expanded(
            child: Text(
              'Status',
              style: Theme.of(context).textTheme.headline6,
            ),
          ),
        ),
        DataColumn(
          label: Expanded(
            child: Text(
              'Created At',
              style: Theme.of(context).textTheme.headline6,
            ),
          ),
        ),
      ],
      rows: const <DataRow>[
        DataRow(
          cells: <DataCell>[
            DataCell(Text('622437367822')),
            DataCell(Text('Montpelier Health')),
            DataCell(Text('SCAN27')),
            DataCell(Text('In Progress')),
            DataCell(Text('2022-06-16 14:01:17.13653')),
          ],
        ),
        DataRow(
          selected: true,
          cells: <DataCell>[
            DataCell(Text('622367437822')),
            DataCell(Text('Sinclair Hospital')),
            DataCell(Text('P24W2')),
            DataCell(Text('In Progress')),
            DataCell(Text('2022-06-16 14:01:17.13653')),
          ],
        ),
        DataRow(
          cells: <DataCell>[
            DataCell(Text('622437873622')),
            DataCell(Text('Montpelier Health')),
            DataCell(Text('SCAN27')),
            DataCell(Text('In Progress')),
            DataCell(Text('2022-06-16 14:01:17.13653')),
          ],
        ),
        DataRow(
          cells: <DataCell>[
            DataCell(Text('622438273672')),
            DataCell(Text('ExpoVital Health')),
            DataCell(Text('TCAN05')),
            DataCell(Text('In Progress')),
            DataCell(Text('2022-06-16 14:01:17.13653')),
          ],
        ),
        DataRow(
          cells: <DataCell>[
            DataCell(Text('622437836722')),
            DataCell(Text('Montpelier Health')),
            DataCell(Text('SCAN27')),
            DataCell(Text('In Progress')),
            DataCell(Text('2022-06-16 14:01:17.13653')),
          ],
        ),
        DataRow(
          cells: <DataCell>[
            DataCell(Text('622437367822')),
            DataCell(Text('Montpelier Health')),
            DataCell(Text('SCAN27')),
            DataCell(Text('In Progress')),
            DataCell(Text('2022-06-16 14:01:17.13653')),
          ],
        ),
      ],
    );
  }
}
