{ pkgs }:

let
  inherit (pkgs) lib;
  app = builtins.fromJSON (builtins.readFile ../mygo.json);
  desktopLibraries = import ./desktop-libraries.nix { inherit pkgs; };
  arch = pkgs.stdenv.hostPlatform.go.GOARCH;

  mathFont = pkgs.fetchurl {
    url = "https://raw.githubusercontent.com/typst/typst-assets/94dcb990edb36311ef85162baa26e4a8205e0bf0/files/fonts/NewCMMath-Book.otf";
    sha256 = "2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95";
  };

  cjdict = pkgs.fetchurl {
    url = "https://raw.githubusercontent.com/unicode-org/icu/release-78.3/icu4c/source/data/brkitr/dictionaries/cjdict.txt";
    sha256 = "e73fd72048981d0cc13e9dc436a7eaba07ffb6eff58c8a59dc75c1df746663a0";
  };
in
pkgs.buildGo127Module {
  pname = app.slug;
  inherit (app) version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../mygo.json
      ../LICENSE
      ../changelogs
      ../cmd
      ../locales
      ../tools
      (lib.fileset.fileFilter (file: !lib.hasSuffix ".png" file.name) ../assets)
      (lib.fileset.fileFilter (
        file:
        !(
          lib.hasSuffix ".ttf" file.name || lib.hasSuffix ".otf" file.name || lib.hasSuffix ".bin" file.name
        )
      ) ../internal)
    ];
  };

  vendorHash = "sha256-Kr6ZhiZ058wN5QazwhuP4Sjidk//L2JKoUK1MxPvuBI=";
  proxyVendor = true;
  env.CGO_ENABLED = 0;

  nativeBuildInputs = [
    pkgs.wrapGAppsHook3
  ];
  buildInputs = [
    pkgs.gtk3
    pkgs.gsettings-desktop-schemas
    pkgs.hicolor-icon-theme
    pkgs.librsvg
  ];
  dontWrapGApps = true;

  nativeCheckInputs = desktopLibraries;

  buildPhase = ''
    runHook preBuild
    export XDG_CACHE_HOME="$TMPDIR/cache"
    export FONTCONFIG_FILE="${pkgs.fontconfig.out}/etc/fonts/fonts.conf"
    mkdir -p "$XDG_CACHE_HOME"
    install -Dm644 ${mathFont} internal/ui/theme/fonts/NewCMMath-Book.otf
    go tool dictionary -src ${cjdict}
    LD_LIBRARY_PATH="${lib.makeLibraryPath desktopLibraries}" \
      go tool icon assets/profile.icon assets/icon.png
    go tool mygo build -platform linux/${arch} -skip-notarize
    runHook postBuild
  '';

  preCheck = ''
    export LD_LIBRARY_PATH="${lib.makeLibraryPath desktopLibraries}"
    export XDG_CACHE_HOME="$TMPDIR/cache"
    export FONTCONFIG_FILE="${pkgs.fontconfig.out}/etc/fonts/fonts.conf"
    mkdir -p "$XDG_CACHE_HOME"
  '';
  checkPhase = ''
    runHook preCheck
    go test ./...
    runHook postCheck
  '';

  installPhase = ''
    runHook preInstall
    install -Dm755 "dist/linux-${arch}/${app.slug}" "$out/libexec/${app.slug}/${app.slug}"
    install -Dm644 "dist/linux-${arch}/${app.slug}.desktop" "$out/share/applications/${app.slug}.desktop"
    install -Dm644 "dist/linux-${arch}/${app.slug}-256x256.png" "$out/share/icons/hicolor/256x256/apps/${app.slug}.png"
    install -Dm644 "dist/linux-${arch}/${app.slug}-512x512.png" "$out/share/icons/hicolor/512x512/apps/${app.slug}.png"
    install -Dm644 "dist/linux-${arch}/${app.slug}.png" "$out/share/icons/hicolor/512x512@2/apps/${app.slug}.png"
    install -Dm644 LICENSE "$out/share/licenses/${app.slug}/LICENSE"
    install -Dm644 internal/ui/theme/fonts/OFL.txt "$out/share/licenses/${app.slug}/Noto-OFL.txt"
    install -Dm644 internal/text/wordseg/LICENSE "$out/share/licenses/${app.slug}/wordseg-LICENSE"
    install -Dm644 internal/text/wordseg/LICENSES/* "$out/share/licenses/${app.slug}/"
    runHook postInstall
  '';

  postFixup = ''
    makeWrapper "$out/libexec/${app.slug}/${app.slug}" "$out/bin/${app.slug}" \
      "''${gappsWrapperArgs[@]}" \
      --prefix PATH : ${lib.makeBinPath [ pkgs.xdg-utils ]} \
      --prefix LD_LIBRARY_PATH : "/run/opengl-driver/lib:${lib.makeLibraryPath desktopLibraries}"
  '';

  meta = {
    description = "Frictionless and minimalist native writing app for desktop";
    homepage = "https://github.com/caelyreth/incipit";
    license = lib.licenses.agpl3Only;
    mainProgram = app.slug;
    platforms = lib.platforms.linux;
  };
}
