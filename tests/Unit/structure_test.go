package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestThePackageHasOneOwnerPerResponsibility freezes the application shapes
// this package is allowed to introduce: none, and no parallel composition
// abstraction either.
func TestThePackageHasOneOwnerPerResponsibility(t *testing.T) {
	t.Parallel()

	root := packageRoot(t)
	forbiddenPaths := []string{
		"app/Actions",
		"app/Data",
		"app/Rules",
		"app/Support",
		"bootstrap/providers.go",
		"routes/api.go",
		"routes/channels.go",
		"public/assets/manifest",
		"public/assets/manifest.json",
	}

	for _, path := range forbiddenPaths {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
			t.Errorf("refused application path %s exists in the package", path)
		} else if !os.IsNotExist(err) {
			t.Errorf("checking refused application path %s: %v", path, err)
		}
	}

	inspectExplicitComposition(t, root)
}

// allowedImports is every import outside the standard library that production
// code may name: the middleware type and the policy the headers are written
// with, the validation errors a refusal is answered with, the view registry the
// script is served from and the escaping the component is drawn with, and the
// component props every kyse component takes.
var allowedImports = map[string]bool{
	"github.com/arandu-io/hesape/http":            true,
	"github.com/arandu-io/hesape/http/middleware": true,
	"github.com/arandu-io/hesape/validation":      true,
	"github.com/arandu-io/hesape/view":            true,
	"github.com/arandu-io/kyse/components":        true,
	"github.com/hyz-is/arandu-turnstile":          true,
}

// TestThePackageIsALibrary holds the shape this repository took when the HTTP
// module of the template was removed: no route, no table, no policy, nothing
// to register, and no import of the framework.
//
// Each of those is a door to something the package deliberately does not do.
// A route would answer requests with no Grant behind them, a table would be a
// migration every installer has to run, and a framework import would make a
// form check depend on the application kernel. The verification needs none of
// them: it is a call the handler makes, with values the handler already has.
func TestThePackageIsALibrary(t *testing.T) {
	t.Parallel()

	root := packageRoot(t)
	for _, path := range []string{
		"module.go", "model.go", "policy.go", "service.go", "views.go",
		"resources", "migrations", "database", "routes",
	} {
		if _, err := os.Lstat(filepath.Join(root, path)); err == nil {
			t.Errorf("%s exists; this package registers nothing, owns no table and decides no authorization", path)
		} else if !os.IsNotExist(err) {
			t.Errorf("checking %s: %v", path, err)
		}
	}

	refusedMethods := map[string]string{
		"Routes":     "a route",
		"Migrations": "a table",
		"Boot":       "a boot step",
		"Publishes":  "a publication",
		"Authorize":  "an authorization decision",
	}
	for _, source := range productionGoFiles(t, root) {
		for _, imported := range source.file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("%s has an unreadable import %s", source.path, imported.Path.Value)
				continue
			}
			first, _, _ := strings.Cut(path, "/")
			if strings.Contains(first, ".") && !allowedImports[path] {
				t.Errorf("%s imports %s; production code may import the standard library and the packages in allowedImports",
					source.path, path)
			}
		}

		ast.Inspect(source.file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.TypeSpec:
				if node.Name.Name == "Module" {
					t.Errorf("%s declares a Module type; there is nothing to register", source.path)
				}
			case *ast.FuncDecl:
				if what, refused := refusedMethods[node.Name.Name]; refused && node.Recv != nil {
					t.Errorf("%s declares %s.%s, which is %s this package does not have",
						source.path, receiverType(node), node.Name.Name, what)
				}
				if node.Recv == nil && node.Name.Name == "init" {
					inspectInit(t, source.path, node)
				}
			}
			return true
		})
	}
}

// inspectInit holds the one thing an init in this package may do: put the
// client behaviour into the view registry, which has to hold it before the
// first page that names it is drawn. Anything else an init did would be work an
// application never asked for, done because a package was imported.
func inspectInit(t *testing.T, path string, init *ast.FuncDecl) {
	t.Helper()

	for _, statement := range init.Body.List {
		expression, ok := statement.(*ast.ExprStmt)
		if !ok {
			t.Errorf("%s: init holds a statement that is not a call; it may only register the script asset", path)
			continue
		}
		call, ok := expression.X.(*ast.CallExpr)
		if !ok {
			t.Errorf("%s: init holds an expression that is not a call", path)
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			t.Errorf("%s: init calls something other than view.RegisterAsset", path)
			continue
		}
		if owner, ok := selector.X.(*ast.Ident); !ok || owner.Name != "view" || selector.Sel.Name != "RegisterAsset" {
			t.Errorf("%s: init calls something other than view.RegisterAsset", path)
		}
	}
}

// inspectExplicitComposition rejects the names that would make registration
// implicit. AppServiceProvider is the deliberate nominal exception: when it is
// present it remains a Module, with no Register lifecycle of its own.
func inspectExplicitComposition(t *testing.T, root string) {
	t.Helper()

	forbiddenFunctions := map[string]bool{
		"DiscoverProviders": true,
		"Providers":         true,
		"RegisterProvider":  true,
		"RegisterProviders": true,
		"ResolveProvider":   true,
		"ResolveProviders":  true,
	}
	appProviderFound := false
	appProviderMethods := map[string]bool{}

	for _, source := range productionGoFiles(t, root) {
		compositionAliases := map[string]bool{}
		for _, imported := range source.file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("%s has an unreadable import %s", source.path, imported.Path.Value)
				continue
			}
			if strings.Contains(path, "/container/") || strings.HasSuffix(path, "/container") {
				t.Errorf("%s imports the container surface %s", source.path, path)
			}
			defaultName := ""
			switch path {
			case "github.com/arandu-io/framework/foundation", "github.com/arandu-io/hesape/foundation":
				defaultName = "foundation"
			case "github.com/arandu-io/framework/kernel":
				defaultName = "kernel"
			}
			if defaultName != "" {
				name := defaultName
				if imported.Name != nil {
					name = imported.Name.Name
				}
				if name == "." || name == "_" {
					t.Errorf("%s hides composition behind the import alias %q", source.path, name)
					continue
				}
				compositionAliases[name] = true
			}
		}

		ast.Inspect(source.file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.TypeSpec:
				if node.Name.Name != "Provider" && !strings.HasSuffix(node.Name.Name, "ServiceProvider") {
					return true
				}
				if node.Name.Name == "AppServiceProvider" {
					appProviderFound = true
					return true
				}
				t.Errorf("%s declares %s, a second composition abstraction", source.path, node.Name.Name)
			case *ast.FuncDecl:
				if node.Recv == nil && forbiddenFunctions[node.Name.Name] {
					t.Errorf("%s declares %s, which makes module registration implicit", source.path, node.Name.Name)
				}
				if receiverType(node) == "AppServiceProvider" {
					appProviderMethods[node.Name.Name] = true
				}
			case *ast.SelectorExpr:
				name, ok := node.X.(*ast.Ident)
				if ok && compositionAliases[name.Name] && node.Sel.Name == "Provider" {
					t.Errorf("%s uses a Provider contract instead of Module", source.path)
				}
			}
			return true
		})
	}

	if !appProviderFound {
		return
	}
	for _, method := range []string{"Name", "Routes"} {
		if !appProviderMethods[method] {
			t.Errorf("AppServiceProvider exists without the Module method %s", method)
		}
	}
	if appProviderMethods["Register"] {
		t.Error("AppServiceProvider declares a container-style Register lifecycle")
	}
}

// inspectServiceBoundary reads the route table rather than fixing a list of
// handler names in the test. Every handler registered there must call a Service

type parsedGoFile struct {
	path string
	file *ast.File
}

func productionGoFiles(t *testing.T, root string) []parsedGoFile {
	t.Helper()

	files := []parsedGoFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") || strings.HasSuffix(entry.Name(), ".kyse.go") {
			return nil
		}

		// The comments are kept because a build constraint is one, and a file
		// the compiler never reads is not part of what this package does.
		// buildable, in audit_test.go, is what asks.
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, parsedGoFile{path: filepath.ToSlash(relative), file: file})
		return nil
	})
	if err != nil {
		t.Fatalf("reading production Go files: %v", err)
	}
	return files
}

// registeredHandlers is every method of the receiver handed to a call that
// registers a route. Only those calls: the guard is mounted with a call that

func receiverType(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return ""
	}
	return namedType(function.Recv.List[0].Type)
}

func namedType(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return namedType(expression.X)
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.IndexExpr:
		return namedType(expression.X)
	case *ast.IndexListExpr:
		return namedType(expression.X)
	default:
		return ""
	}
}

func packageRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("finding package root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// TestTheGuidesTeachALibrary keeps the documents an installer and a maintainer
// read in step with the package that exists. The template this package came
// from taught a module that is constructed, registered and migrated, and a
// guide that still taught any of it would send the next reader to write the
// wiring this package refused.
func TestTheGuidesTeachALibrary(t *testing.T) {
	t.Parallel()

	root := packageRoot(t)
	guides := map[string][]string{
		"README.md": {
			"go get github.com/hyz-is/arandu-turnstile", "turnstile.New(", "turnstile.Headers(",
			"turnstile.Widget(", "captcha.Check(", "turnstile.FieldName", "aru view:build",
		},
		"AGENTS.md": {"view.RegisterAsset", "TestThePackageIsALibrary", "aru view:build"},
		".agents/skills/turnstile-package/SKILL.md": {
			"go get github.com/hyz-is/arandu-turnstile", "turnstile.New(", "turnstile.Headers(",
			"turnstile.Widget(", "captcha.Check(", "turnstile.TestSiteKey",
		},
		".agents/skills/turnstile-release/SKILL.md": {"TestTheModuleRequiresNothingButHesapeAndKyse", "aru view:build"},
		".agents/skills/README.md":                  {"turnstile-package", "turnstile-release", "turnstile-vault-notes"},
	}
	retired := []string{
		"turnstileModule", "k.Register(", "TurnstileService", "TurnstilePolicy", "Turnstiles(",
		"model:build", "vendor:publish", "/auth/login",
	}
	bootstrapFile := regexp.MustCompile(`bootstrap/[A-Za-z0-9_.-]+\.go`)

	for path, required := range guides {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		text := string(body)
		for _, want := range required {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not teach %q", path, want)
			}
		}
		for _, stale := range retired {
			if strings.Contains(text, stale) {
				t.Errorf("%s still teaches the template's module through %q", path, stale)
			}
		}
		for _, referenced := range bootstrapFile.FindAllString(text, -1) {
			if referenced != "bootstrap/app.go" {
				t.Errorf("%s introduces a second composition file %s", path, referenced)
			}
		}
	}
}

// TestEverySkillIsNamedAfterItsDirectory holds the one line of a skill that a
// tool reads as data: a skill whose frontmatter name and directory differ is
// skipped, and nothing says so. It also holds which skill an application
// receives -- the package skill, and no other.
func TestEverySkillIsNamedAfterItsDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(packageRoot(t), ".agents", "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the skills: %v", err)
	}
	found := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		found++
		body, err := os.ReadFile(filepath.Join(dir, entry.Name(), "SKILL.md"))
		if err != nil {
			t.Errorf("%s has no SKILL.md: %v", entry.Name(), err)
			continue
		}
		text := string(body)
		if !strings.HasPrefix(text, "---\nname: "+entry.Name()+"\n") {
			t.Errorf("%s/SKILL.md does not open with name: %s", entry.Name(), entry.Name())
		}
		forApplications := strings.Contains(text, "\nmetadata:\n  audience: app\n")
		if want := entry.Name() == "turnstile-package"; forApplications != want {
			t.Errorf("%s: audience app = %v, want %v", entry.Name(), forApplications, want)
		}
	}
	if found != 3 {
		t.Errorf("found %d skills, want turnstile-package, turnstile-release and turnstile-vault-notes", found)
	}
}
