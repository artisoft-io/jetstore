import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:http/http.dart' as http;
import 'package:provider/provider.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/screens/login.dart';

void main() {
  runApp(const JetsClient());
}

class JetsClient extends StatelessWidget {
  const JetsClient({super.key});

  // This widget is the root of your application.
  @override
  Widget build(BuildContext context) {
    return AdaptiveTheme(
      light: ThemeData(
        brightness: Brightness.light,
        colorSchemeSeed: const Color.fromRGBO(118, 219, 21, 1.0)
      ),
      dark: ThemeData(
        brightness: Brightness.dark,
        colorSchemeSeed: const Color.fromRGBO(118, 219, 21, 1.0)
      ),
      initial: AdaptiveThemeMode.light,
      builder: (theme, darkTheme) => MultiProvider(
      providers: [
        // // In this sample app, CatalogModel never changes, so a simple Provider
        // // is sufficient.
        // Provider(create: (context) => CatalogModel()),
        // // CartModel is implemented as a ChangeNotifier, which calls for the use
        // // of ChangeNotifierProvider. Moreover, CartModel depends
        // // on CatalogModel, so a ProxyProvider is needed.
        // ChangeNotifierProxyProvider<CatalogModel, CartModel>(
        //   create: (context) => CartModel(),
        //   update: (context, catalog, cart) {
        //     if (cart == null) throw ArgumentError.notNull('cart');
        //     cart.catalog = catalog;
        //     return cart;
        //   },
        // ),
        // User model for login
        Provider(create: (context) => UserModel()),
        // http Client
        Provider(create: (context) => http.Client()),
      ],
      child: MaterialApp(
        title: 'JetStore Client',
        theme: theme,
        initialRoute: '/',
        routes: {
          '/': (context) => const LoginScreen(),
          // '/catalog': (context) => const MyCatalog(),
          // '/cart': (context) => const MyCart(),
        },
      ),
    ),
    );
  }
}
