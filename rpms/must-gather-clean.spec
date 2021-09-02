#this is a template spec and actual spec will be generated
#debuginfo not supported with Go
%global debug_package %{nil}
%global _enable_debug_package 0
%global __os_install_post /usr/lib/rpm/brp-compress %{nil}
%global package_name must-gather-clean
%global product_name must-gather-clean
%global golang_version ${GOLANG_VERSION}
%global golang_version_nodot ${GOLANG_VERSION_NODOT}
%global mgc_version ${MGC_RPM_VERSION}
%global mgc_release ${MGC_RELEASE}
%global git_commit  ${GIT_COMMIT}
%global mgc_cli_version v%{mgc_version}
%global source_dir must-gather-clean-%{mgc_version}-%{mgc_release}
%global source_tar %{source_dir}.tar.gz
%global gopath  %{_builddir}/gocode
%global _missing_build_ids_terminate_build 0

Name:           %{package_name}
Version:        %{mgc_version}
Release:        %{mgc_release}%{?dist}
Summary:        %{product_name} client must-gather-clean CLI binary
License:        ASL 2.0
URL:            https://github.com/openshift/must-gather-clean/tree/%{mgc_cli_version}

Source0:        %{source_tar}
BuildRequires:  gcc
BuildRequires:  golang >= %{golang_version}
Provides:       %{package_name} = %{mgc_version}
Obsoletes:      %{package_name} <= %{mgc_version}

%description
must-gather-clean is a tool for obfuscating sensitive data from must-gather dumps.

%prep
%setup -q -n %{source_dir}

%build
export GITCOMMIT="%{git_commit}"
mkdir -p %{gopath}/src/github.com/openshift
ln -s "$(pwd)" %{gopath}/src/github.com/openshift/must-gather-clean
export GOPATH=%{gopath}
cd %{gopath}/src/github.com/openshift/must-gather-clean
go mod edit -go=%{golang_version}
%ifarch x86_64
# go test -race is not supported on all arches
GOFLAGS='-mod=vendor' make test
%endif
make prepare-release
echo "%{mgc_version}" > dist/release/VERSION
unlink %{gopath}/src/github.com/openshift/must-gather-clean

%install
mkdir -p %{buildroot}/%{_bindir}
install -m 0755 dist/bin/linux-`go env GOARCH`/must-gather-clean %{buildroot}%{_bindir}/must-gather-clean
mkdir -p %{buildroot}%{_datadir}
install -d %{buildroot}%{_datadir}/%{name}-redistributable
install -p -m 755 dist/release/must-gather-clean-linux-amd64 %{buildroot}%{_datadir}/%{name}-redistributable/must-gather-clean-linux-amd64
install -p -m 755 dist/release/must-gather-clean-linux-arm64 %{buildroot}%{_datadir}/%{name}-redistributable/must-gather-clean-linux-arm64
install -p -m 755 dist/release/must-gather-clean-darwin-amd64 %{buildroot}%{_datadir}/%{name}-redistributable/must-gather-clean-darwin-amd64
install -p -m 755 dist/release/must-gather-clean-windows-amd64.exe %{buildroot}%{_datadir}/%{name}-redistributable/must-gather-clean-windows-amd64.exe
cp -avrf dist/release/must-gather-clean*.tar.gz %{buildroot}%{_datadir}/%{name}-redistributable
cp -avrf dist/release/must-gather-clean*.zip %{buildroot}%{_datadir}/%{name}-redistributable
cp -avrf dist/release/SHA256_SUM %{buildroot}%{_datadir}/%{name}-redistributable
cp -avrf dist/release/VERSION %{buildroot}%{_datadir}/%{name}-redistributable

%files
%license LICENSE
%{_bindir}/must-gather-clean

%package redistributable
Summary:        %{product_name} client CLI binaries for Linux, macOS and Windows
BuildRequires:  gcc
BuildRequires:  golang >= %{golang_version}
Provides:       %{package_name}-redistributable = %{mgc_version}
Obsoletes:      %{package_name}-redistributable <= %{mgc_version}

%description redistributable
%{product_name} client must-gather-clean cross platform binaries for Linux, macOS and Windows.

%files redistributable
%license LICENSE
%dir %{_datadir}/%{name}-redistributable
%{_datadir}/%{name}-redistributable/must-gather-clean-linux-amd64
%{_datadir}/%{name}-redistributable/must-gather-clean-linux-amd64.tar.gz
%{_datadir}/%{name}-redistributable/must-gather-clean-linux-arm64
%{_datadir}/%{name}-redistributable/must-gather-clean-linux-arm64.tar.gz
%{_datadir}/%{name}-redistributable/must-gather-clean-darwin-amd64
%{_datadir}/%{name}-redistributable/must-gather-clean-darwin-amd64.tar.gz
%{_datadir}/%{name}-redistributable/must-gather-clean-windows-amd64.exe
%{_datadir}/%{name}-redistributable/must-gather-clean-windows-amd64.exe.zip
%{_datadir}/%{name}-redistributable/SHA256_SUM
%{_datadir}/%{name}-redistributable/VERSION

