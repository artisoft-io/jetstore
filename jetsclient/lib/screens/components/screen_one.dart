import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/screen_config.dart';

class ScreenOne extends StatefulWidget {
  const ScreenOne(
      {super.key, required this.tablePath, required this.screenConfig});

  final JetsRouteData tablePath;
  final ScreenConfig screenConfig;


  @override
  State<ScreenOne> createState() => ScreenOneState();
}

class ScreenOneState extends State<ScreenOne> {

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: appBar(context, widget.screenConfig.appBarLabel,
          showLogout: widget.screenConfig.showLogout),
      body: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Flexible(
            flex: 1,
            fit: FlexFit.tight,
            child: Column(children: [
              const SizedBox(height: defaultPadding),
              Expanded(
                flex: 1,
                child: Image.asset(widget.screenConfig.leftBarLogo),
              ),
              const SizedBox(height: defaultPadding),
              Expanded(
                  flex: 8,
                  child: ListView.separated(
                    padding: const EdgeInsets.all(defaultPadding),
                    itemCount: widget.screenConfig.menuEntries.length,
                    itemBuilder: (BuildContext context, int index) {
                      return ElevatedButton(
                        style: ElevatedButton.styleFrom(
                          foregroundColor: Theme.of(context)
                              .colorScheme
                              .onSecondaryContainer,
                          backgroundColor:
                              Theme.of(context).colorScheme.secondaryContainer,
                        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
                        onPressed: () => JetsRouterDelegate()(JetsRouteData(
                            widget.screenConfig.menuEntries[index].routePath)),
                        child: Center(
                            child: Text(widget.screenConfig.menuEntries[index].label)),
                      );
                    },
                    separatorBuilder: (BuildContext context, int index) =>
                        const Divider(),
                  ))
            ]),
          ),
          Flexible(
            flex: 5,
            fit: FlexFit.tight,
            child:
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const SizedBox(height: 2 * defaultPadding),
              Flexible(
                flex: 1,
                fit: FlexFit.tight,
                child: Text(
                  widget.screenConfig.title,
                  style: Theme.of(context).textTheme.headline4,
                ),
              ),
              Flexible(
                flex: 8,
                fit: FlexFit.tight,
                child: JetsDataTableWidget(
                    tablePath: widget.tablePath, tableConfig: widget.screenConfig.tableConfig),
              ),
            ]),
          ),
        ],
      ),
    );
  }
}
