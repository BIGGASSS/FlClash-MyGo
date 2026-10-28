// Isolated desktop navigation widgets from the pinned FlClash source.
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/physics.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/state.dart';
import 'package:fl_clash/ui/ui.dart';
import 'package:fl_clash/ui/icons/icons.dart';

import 'port_reference_test.dart' show ReferenceApp, ReferenceBinding;

const destinations = [
  NavigationDockDestination(glyph:AppGlyphs.dashboard,label:'Dashboard'),
  NavigationDockDestination(glyph:AppGlyphs.proxies,label:'Proxies'),
  NavigationDockDestination(glyph:AppGlyphs.profiles,label:'Profiles'),
  NavigationDockDestination(glyph:AppGlyphs.tools,label:'Tools'),
];

void main() {
  ReferenceBinding();
  setUpAll(() async {
    final sdk=Platform.environment['FLUTTER_ROOT']!;
    for (final item in [('Roboto-Regular.ttf',FontWeight.w400),('Roboto-Medium.ttf',FontWeight.w500),('Roboto-Bold.ttf',FontWeight.w700)]) {
      final bytes=await File('$sdk/bin/cache/artifacts/material_fonts/${item.$1}').readAsBytes();
      await (FontLoader('Roboto')..addFont(Future.value(ByteData.sublistView(bytes)))).load();
    }
  });
  test('export navigation spring curves',() async {
    final rows=<Object>[];
    for (final spec in [(2*math.pi/.4,.85),(2*math.pi/.25,1.0),(math.sqrt(311),.593),(32.0,.9),(10.0,.4),(12.0,1.3)]) {
      final spring=SpringDescription.withDampingRatio(mass:1,stiffness:spec.$1*spec.$1,ratio:spec.$2);
      final simulation=SpringSimulation(spring,-1.2,2.3,.7,tolerance:Tolerance(distance:.001,velocity:.001*spec.$1));
      rows.add({'frequency':spec.$1,'ratio':spec.$2,'samples':[
        for (final ms in [0,16,33,66,100,200,400,800,1600,3200]) {'ms':ms,'value':simulation.isDone(ms/1000)?2.3:simulation.x(ms/1000),'velocity':simulation.isDone(ms/1000)?0.0:simulation.dx(ms/1000)},
      ]});
    }
    await File('${Platform.environment['FLCLASH_REFERENCE_DIR']!}/navigation-springs.json').writeAsString(jsonEncode(rows));
  });
  testWidgets('export navigation velocities',(tester) async {
    final rows=<Object>[];
    for(final points in [
      [for(var i=0;i<6;i++) (i*16,i*8.0)],
      [(0,0.0),(16,5.0),(32,20.0),(48,45.0)],
      [(0,50.0),(16,45.0),(32,35.0),(48,20.0)],
      [(0,0.0),(100,30.0)],
      [(0,0.0),(16,30.0)],
      [(0,0.0),(0,10.0),(0,30.0)],
      [for(var i=0;i<40;i++) (i*4,20*math.sin(i/6))],
    ]) {
      final tracker=VelocityTracker.withKind(ui.PointerDeviceKind.mouse);
      for(final p in points) {tracker.addPosition(Duration(milliseconds:p.$1),Offset(p.$2,0));}
      rows.add({'times':[for(final p in points)p.$1],'positions':[for(final p in points)p.$2],'velocity':tracker.getVelocity().pixelsPerSecond.dx});
    }
    await tester.runAsync(() async {await File('${Platform.environment['FLCLASH_REFERENCE_DIR']!}/navigation-velocities.json').writeAsString(jsonEncode(rows));});
  });
  for (final name in ['dock-380','dock-600','dock-scaled-up','dock-empty','dock-docked','sidebar-laptop','sidebar-overlay']) {
    for (final dark in [false,true]) {
      testWidgets('$name ${dark?'dark':'light'}',(tester) async {
        final width=name.startsWith('sidebar')?820.0:name=='dock-600'?600.0:380.0;
        final scale=name=='dock-scaled-up'?1.4:1.0;
        final size=Size(width,name.startsWith('sidebar')?800:400);
        tester.view.physicalSize=size;tester.view.devicePixelRatio=1;
        addTearDown(tester.view.resetPhysicalSize);addTearDown(tester.view.resetDevicePixelRatio);
        final container=ProviderContainer(overrides:[
          appSettingProvider.overrideWithBuild((_,_)=>const AppSettingProps(locale:'en')),
          themeSettingProvider.overrideWithBuild((_,_)=>ThemeProps(primaryColor:0xff007aff,themeMode:dark?ThemeMode.dark:ThemeMode.light)),
        ]);
        globalState.container=container;
        addTearDown(container.dispose);
        container.read(viewSizeProvider.notifier).value=size;
        final items=name=='dock-empty'?[destinations[0],destinations[2],destinations[3]]:destinations;
        final child=Builder(builder:(context)=>ColoredBox(color:SurfaceLevel.page.colorOf(Theme.of(context).colorScheme),child:
          name.startsWith('sidebar')?Row(children:[NavigationSidebar(destinations:const[
            SidebarDestination(glyph:AppGlyphs.dashboard,label:'Dashboard'),SidebarDestination(glyph:AppGlyphs.proxies,label:'Proxies'),SidebarDestination(glyph:AppGlyphs.profiles,label:'Profiles'),SidebarDestination(glyph:AppGlyphs.connections,label:'Live connections'),SidebarDestination(glyph:AppGlyphs.tools,label:'Tools'),
          ],selectedIndex:0,expanded:false,onSelected:(_){},onToggle:null,windowControls:Size.zero),const Expanded(child:SizedBox())]):Align(alignment:Alignment.bottomCenter,child:
            name=='dock-docked'?NavigationBar(selectedIndex:0,onDestinationSelected:(_){},destinations:[for(final (index,item) in items.indexed)NavigationDestination(icon:GlyphIcon(item.glyph,fill:index==0?1:0),label:item.label)]):NavigationDock(destinations:items,selectedIndex:0,onSelected:(_){}),
          ),
        ));
        await tester.pumpWidget(UncontrolledProviderScope(container:container,child:ReferenceApp(child:child,dark:dark,selected:0,scale:scale,bare:true)));
        await tester.pump();await tester.pump(const Duration(seconds:1));
        if(name=='sidebar-overlay') {await tester.tap(find.descendant(of:find.byType(NavigationSidebar),matching:find.byType(InkWell)).first);await tester.pump();await tester.pump(const Duration(milliseconds:300));}
        final metrics={
          'dock':[for(final e in find.byType(FloatingNavigationBar).evaluate()) (() {final r=tester.getRect(find.byElementPredicate((v)=>v==e));return [r.left,r.top,r.width,r.height];})()],
          'labels':{for(final item in items) item.label:[for(final e in find.text(item.label).evaluate()) (() {final r=tester.getRect(find.byElementPredicate((v)=>v==e));return [r.left,r.top,r.width,r.height];})()]},
        };
        await tester.runAsync(() async {
          final boundary=tester.renderObject<RenderRepaintBoundary>(find.byKey(const ValueKey('snapshot')));
          final image=await boundary.toImage(pixelRatio:1);
          final bytes=await image.toByteData(format:ui.ImageByteFormat.png);
          final dir=Platform.environment['FLCLASH_REFERENCE_DIR']!;
          await File('$dir/$name-${dark?'dark':'light'}.png').writeAsBytes(bytes!.buffer.asUint8List());
          await File('$dir/$name-${dark?'dark':'light'}.json').writeAsString(jsonEncode(metrics));
          image.dispose();
        });
        await tester.pumpWidget(const SizedBox.shrink());await tester.pump(const Duration(seconds:1));
      });
    }
  }
}
