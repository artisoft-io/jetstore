import 'package:flutter/material.dart';

class MyDataTableSampleWidget extends StatelessWidget {
  const MyDataTableSampleWidget({super.key});

  @override
  Widget build(BuildContext context) {
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
