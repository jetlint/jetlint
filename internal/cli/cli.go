// Package cli implements the command-line interface for the jetlint binary.
// Keeping it out of package main lets tests exercise Run in-process without
// a build step.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Imported to anchor the wrapper-API dependency: the binary statically
	// links against the fork's exported packages. The reference is here
	// (not in main) so the architecture test sees rule-package-shaped imports.
	_ "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/config"
	"github.com/jetlint/jetlint/internal/daemon"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/format"
	"github.com/jetlint/jetlint/internal/project"
	"github.com/jetlint/jetlint/internal/rules"
	"github.com/jetlint/jetlint/internal/rules/arraycallbackreturn"
	"github.com/jetlint/jetlint/internal/rules/awaitthenable"
	"github.com/jetlint/jetlint/internal/rules/consistentreturn"
	"github.com/jetlint/jetlint/internal/rules/consistenttypeexports"
	"github.com/jetlint/jetlint/internal/rules/constructorsuper"
	"github.com/jetlint/jetlint/internal/rules/defaultcaselast"
	"github.com/jetlint/jetlint/internal/rules/dotnotation"
	"github.com/jetlint/jetlint/internal/rules/fordirection"
	"github.com/jetlint/jetlint/internal/rules/getterreturn"
	"github.com/jetlint/jetlint/internal/rules/guardforin"
	"github.com/jetlint/jetlint/internal/rules/namingconvention"
	"github.com/jetlint/jetlint/internal/rules/noadjacentspacesinregex"
	"github.com/jetlint/jetlint/internal/rules/noalert"
	"github.com/jetlint/jetlint/internal/rules/noapproximativenumericconstant"
	"github.com/jetlint/jetlint/internal/rules/noarguments"
	"github.com/jetlint/jetlint/internal/rules/noarrayindexkey"
	"github.com/jetlint/jetlint/internal/rules/noarraydelete"
	"github.com/jetlint/jetlint/internal/rules/noassigninexpressions"
	"github.com/jetlint/jetlint/internal/rules/noasyncpromiseexecutor"
	"github.com/jetlint/jetlint/internal/rules/noawaitinloop"
	"github.com/jetlint/jetlint/internal/rules/nobasetotostring"
	"github.com/jetlint/jetlint/internal/rules/nobitwiseoperators"
	"github.com/jetlint/jetlint/internal/rules/nocatchassign"
	"github.com/jetlint/jetlint/internal/rules/nocommenttext"
	"github.com/jetlint/jetlint/internal/rules/nochildrenprop"
	"github.com/jetlint/jetlint/internal/rules/noclassassign"
	"github.com/jetlint/jetlint/internal/rules/nocommaoperator"
	"github.com/jetlint/jetlint/internal/rules/nocomparenegzero"
	"github.com/jetlint/jetlint/internal/rules/nocondassign"
	"github.com/jetlint/jetlint/internal/rules/noconfusinglabels"
	"github.com/jetlint/jetlint/internal/rules/noconfusingvoidexpression"
	"github.com/jetlint/jetlint/internal/rules/noconfusingvoidtype"
	"github.com/jetlint/jetlint/internal/rules/noconsole"
	"github.com/jetlint/jetlint/internal/rules/noconstantbinaryexpression"
	"github.com/jetlint/jetlint/internal/rules/noconstantcondition"
	"github.com/jetlint/jetlint/internal/rules/noconstantmathminmaxclamp"
	"github.com/jetlint/jetlint/internal/rules/noconstassign"
	"github.com/jetlint/jetlint/internal/rules/noconstenum"
	"github.com/jetlint/jetlint/internal/rules/noconstructorreturn"
	"github.com/jetlint/jetlint/internal/rules/nocontrolregex"
	"github.com/jetlint/jetlint/internal/rules/nodebugger"
	"github.com/jetlint/jetlint/internal/rules/nodeprecated"
	"github.com/jetlint/jetlint/internal/rules/nodeprecatedimports"
	"github.com/jetlint/jetlint/internal/rules/nodocumentcookie"
	"github.com/jetlint/jetlint/internal/rules/nodocumentimportinpage"
	"github.com/jetlint/jetlint/internal/rules/nodoubleequals"
	"github.com/jetlint/jetlint/internal/rules/nodupeargs"
	"github.com/jetlint/jetlint/internal/rules/nodupeclassmembers"
	"github.com/jetlint/jetlint/internal/rules/nodupeelseif"
	"github.com/jetlint/jetlint/internal/rules/nodupekeys"
	"github.com/jetlint/jetlint/internal/rules/noduplicatecase"
	"github.com/jetlint/jetlint/internal/rules/noduplicateimports"
	"github.com/jetlint/jetlint/internal/rules/noduplicatejsxprops"
	"github.com/jetlint/jetlint/internal/rules/noduplicateprivateclassmembers"
	"github.com/jetlint/jetlint/internal/rules/noduplicatetesthooks"
	"github.com/jetlint/jetlint/internal/rules/noduplicatetypeconstituents"
	"github.com/jetlint/jetlint/internal/rules/noempty"
	"github.com/jetlint/jetlint/internal/rules/noemptycharacterclass"
	"github.com/jetlint/jetlint/internal/rules/noemptyinterface"
	"github.com/jetlint/jetlint/internal/rules/noemptypattern"
	"github.com/jetlint/jetlint/internal/rules/noemptysource"
	"github.com/jetlint/jetlint/internal/rules/noemptytypeparameters"
	"github.com/jetlint/jetlint/internal/rules/noevolvingtypes"
	"github.com/jetlint/jetlint/internal/rules/noexassign"
	"github.com/jetlint/jetlint/internal/rules/noexplicitany"
	"github.com/jetlint/jetlint/internal/rules/noexportsintest"
	"github.com/jetlint/jetlint/internal/rules/noextrabooleancast"
	"github.com/jetlint/jetlint/internal/rules/noextranonnullassertion"
	"github.com/jetlint/jetlint/internal/rules/nofallthrough"
	"github.com/jetlint/jetlint/internal/rules/noflatmapidentity"
	"github.com/jetlint/jetlint/internal/rules/nofloatingpromises"
	"github.com/jetlint/jetlint/internal/rules/nofocusedtests"
	"github.com/jetlint/jetlint/internal/rules/noforeach"
	"github.com/jetlint/jetlint/internal/rules/noforinarray"
	"github.com/jetlint/jetlint/internal/rules/nofuncassign"
	"github.com/jetlint/jetlint/internal/rules/nofunctionassign"
	"github.com/jetlint/jetlint/internal/rules/noglobalassign"
	"github.com/jetlint/jetlint/internal/rules/noglobaldirnamefilename"
	"github.com/jetlint/jetlint/internal/rules/noglobalisfinite"
	"github.com/jetlint/jetlint/internal/rules/noglobalisnan"
	"github.com/jetlint/jetlint/internal/rules/noheadimportindocument"
	"github.com/jetlint/jetlint/internal/rules/noimplicitanylet"
	"github.com/jetlint/jetlint/internal/rules/noimpliedeval"
	"github.com/jetlint/jetlint/internal/rules/noimportassign"
	"github.com/jetlint/jetlint/internal/rules/noinitializerwithdefinite"
	"github.com/jetlint/jetlint/internal/rules/noinnerdeclarations"
	"github.com/jetlint/jetlint/internal/rules/noinstanceofarray"
	"github.com/jetlint/jetlint/internal/rules/adjacentoverloadsignatures"
	"github.com/jetlint/jetlint/internal/rules/nomisusednew"
	"github.com/jetlint/jetlint/internal/rules/noinvalidbuiltininstantiation"
	"github.com/jetlint/jetlint/internal/rules/noinvalidregexp"
	"github.com/jetlint/jetlint/internal/rules/noirregularwhitespace"
	"github.com/jetlint/jetlint/internal/rules/nolabelvar"
	"github.com/jetlint/jetlint/internal/rules/nolossofprecision"
	"github.com/jetlint/jetlint/internal/rules/nomeaninglessvoidoperator"
	"github.com/jetlint/jetlint/internal/rules/nomisleadingcharacterclass"
	"github.com/jetlint/jetlint/internal/rules/nomisplacedassertion"
	"github.com/jetlint/jetlint/internal/rules/nomisrefactoredshorthandassign"
	"github.com/jetlint/jetlint/internal/rules/nomisusedpromises"
	"github.com/jetlint/jetlint/internal/rules/nomisusedspread"
	"github.com/jetlint/jetlint/internal/rules/nomixedenums"
	"github.com/jetlint/jetlint/internal/rules/nonestedcomponentdefinitions"
	"github.com/jetlint/jetlint/internal/rules/nonewnativenonconstructor"
	"github.com/jetlint/jetlint/internal/rules/nonextasyncclientcomponent"
	"github.com/jetlint/jetlint/internal/rules/nonnullabletypeassertionstyle"
	"github.com/jetlint/jetlint/internal/rules/nonodejsmodules"
	"github.com/jetlint/jetlint/internal/rules/nononnullassertedoptionalchain"
	"github.com/jetlint/jetlint/internal/rules/nononoctaldecimalescape"
	"github.com/jetlint/jetlint/internal/rules/noobjcalls"
	"github.com/jetlint/jetlint/internal/rules/nooctalescape"
	"github.com/jetlint/jetlint/internal/rules/noprecisionloss"
	"github.com/jetlint/jetlint/internal/rules/noprivateimports"
	"github.com/jetlint/jetlint/internal/rules/noprocessglobal"
	"github.com/jetlint/jetlint/internal/rules/nopromiseexecutorreturn"
	"github.com/jetlint/jetlint/internal/rules/noprototypebuiltins"
	"github.com/jetlint/jetlint/internal/rules/noqwikusevisibletask"
	"github.com/jetlint/jetlint/internal/rules/noreactforwardref"
	"github.com/jetlint/jetlint/internal/rules/noreactpropassignments"
	"github.com/jetlint/jetlint/internal/rules/noreactspecificprops"
	"github.com/jetlint/jetlint/internal/rules/noredundanttypeconstituents"
	"github.com/jetlint/jetlint/internal/rules/noredundantusestrict"
	"github.com/jetlint/jetlint/internal/rules/norenderreturnvalue"
	"github.com/jetlint/jetlint/internal/rules/norestrictedelements"
	"github.com/jetlint/jetlint/internal/rules/noselfassign"
	"github.com/jetlint/jetlint/internal/rules/noselfcompare"
	"github.com/jetlint/jetlint/internal/rules/nosetterreturn"
	"github.com/jetlint/jetlint/internal/rules/noshadowrestrictednames"
	"github.com/jetlint/jetlint/internal/rules/noskippedtests"
	"github.com/jetlint/jetlint/internal/rules/nosoliddestructuredprops"
	"github.com/jetlint/jetlint/internal/rules/nosparsearrays"
	"github.com/jetlint/jetlint/internal/rules/nostaticonlyclass"
	"github.com/jetlint/jetlint/internal/rules/nostringcasemismatch"
	"github.com/jetlint/jetlint/internal/rules/nosuperwithoutextends"
	"github.com/jetlint/jetlint/internal/rules/noswitchdeclarations"
	"github.com/jetlint/jetlint/internal/rules/notemplatecurlyinstring"
	"github.com/jetlint/jetlint/internal/rules/nothenproperty"
	"github.com/jetlint/jetlint/internal/rules/nothisbeforesuper"
	"github.com/jetlint/jetlint/internal/rules/notsignore"
	"github.com/jetlint/jetlint/internal/rules/noimportcycles"
	"github.com/jetlint/jetlint/internal/rules/noredeclare"
	"github.com/jetlint/jetlint/internal/rules/notypeonlyimportattributes"
	"github.com/jetlint/jetlint/internal/rules/nounassignedvariables"
	"github.com/jetlint/jetlint/internal/rules/noundeclareddependencies"
	"github.com/jetlint/jetlint/internal/rules/noundef"
	"github.com/jetlint/jetlint/internal/rules/nounexpectedmultiline"
	"github.com/jetlint/jetlint/internal/rules/nounmodifiedloopcondition"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarybooleanliteralcompare"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarycondition"
	"github.com/jetlint/jetlint/internal/rules/nounnecessaryqualifier"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytemplateexpression"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypearguments"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeassertion"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeconversion"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeparameters"
	"github.com/jetlint/jetlint/internal/rules/nounreachable"
	"github.com/jetlint/jetlint/internal/rules/nounreachableloop"
	"github.com/jetlint/jetlint/internal/rules/nounreachablesuper"
	"github.com/jetlint/jetlint/internal/rules/nounresolvedimports"
	"github.com/jetlint/jetlint/internal/rules/nounsafeargument"
	"github.com/jetlint/jetlint/internal/rules/nounsafeassignment"
	"github.com/jetlint/jetlint/internal/rules/nounsafecall"
	"github.com/jetlint/jetlint/internal/rules/nounsafedeclarationmerging"
	"github.com/jetlint/jetlint/internal/rules/nounsafeenumcomparison"
	"github.com/jetlint/jetlint/internal/rules/nounsafefinally"
	"github.com/jetlint/jetlint/internal/rules/nounsafememberaccess"
	"github.com/jetlint/jetlint/internal/rules/nounsafenegation"
	"github.com/jetlint/jetlint/internal/rules/nounsafeoptionalchaining"
	"github.com/jetlint/jetlint/internal/rules/nounsafereturn"
	"github.com/jetlint/jetlint/internal/rules/nounsafetypeassertion"
	"github.com/jetlint/jetlint/internal/rules/nounsafeunaryminus"
	"github.com/jetlint/jetlint/internal/rules/nounusedexpressions"
	"github.com/jetlint/jetlint/internal/rules/nounusedfunctionparameters"
	"github.com/jetlint/jetlint/internal/rules/nounusedimports"
	"github.com/jetlint/jetlint/internal/rules/nounusedlabels"
	"github.com/jetlint/jetlint/internal/rules/nounusedprivateclassmembers"
	"github.com/jetlint/jetlint/internal/rules/nounusedvars"
	"github.com/jetlint/jetlint/internal/rules/nousebeforedefine"
	"github.com/jetlint/jetlint/internal/rules/nouselessbackreference"
	"github.com/jetlint/jetlint/internal/rules/nouselesscatch"
	"github.com/jetlint/jetlint/internal/rules/nouselesscatchbinding"
	"github.com/jetlint/jetlint/internal/rules/nouselesscontinue"
	"github.com/jetlint/jetlint/internal/rules/nouselessdefaultassignment"
	"github.com/jetlint/jetlint/internal/rules/nouselessemptyexport"
	"github.com/jetlint/jetlint/internal/rules/nouselessescapeinstring"
	"github.com/jetlint/jetlint/internal/rules/nouselesslabel"
	"github.com/jetlint/jetlint/internal/rules/nouselessrename"
	"github.com/jetlint/jetlint/internal/rules/nouselessstringconcat"
	"github.com/jetlint/jetlint/internal/rules/nouselessstringraw"
	"github.com/jetlint/jetlint/internal/rules/nouselessswitchcase"
	"github.com/jetlint/jetlint/internal/rules/nouselessternary"
	"github.com/jetlint/jetlint/internal/rules/nouselessundefinedinitialization"
	"github.com/jetlint/jetlint/internal/rules/novar"
	"github.com/jetlint/jetlint/internal/rules/novoidelementswithchildren"
	"github.com/jetlint/jetlint/internal/rules/novoidtypereturn"
	"github.com/jetlint/jetlint/internal/rules/novuedataobjectdeclaration"
	"github.com/jetlint/jetlint/internal/rules/novueduplicatekeys"
	"github.com/jetlint/jetlint/internal/rules/novuereservedkeys"
	"github.com/jetlint/jetlint/internal/rules/novuereservedprops"
	"github.com/jetlint/jetlint/internal/rules/novuesetuppropsreactivityloss"
	"github.com/jetlint/jetlint/internal/rules/nowith"
	"github.com/jetlint/jetlint/internal/rules/onlythrowerror"
	"github.com/jetlint/jetlint/internal/rules/preferdestructuring"
	"github.com/jetlint/jetlint/internal/rules/preferfind"
	"github.com/jetlint/jetlint/internal/rules/preferincludes"
	"github.com/jetlint/jetlint/internal/rules/prefernamespacekeyword"
	"github.com/jetlint/jetlint/internal/rules/prefernullishcoalescing"
	"github.com/jetlint/jetlint/internal/rules/preferoptionalchain"
	"github.com/jetlint/jetlint/internal/rules/preferpromiserejecterrors"
	"github.com/jetlint/jetlint/internal/rules/preferreadonly"
	"github.com/jetlint/jetlint/internal/rules/preferreadonlyparametertypes"
	"github.com/jetlint/jetlint/internal/rules/preferreducetypeparameter"
	"github.com/jetlint/jetlint/internal/rules/preferregexpexec"
	"github.com/jetlint/jetlint/internal/rules/preferreturnthistype"
	"github.com/jetlint/jetlint/internal/rules/preferstringstartsendswith"
	"github.com/jetlint/jetlint/internal/rules/promisefunctionasync"
	"github.com/jetlint/jetlint/internal/rules/relatedgettersetterpairs"
	"github.com/jetlint/jetlint/internal/rules/requirearraysortcompare"
	"github.com/jetlint/jetlint/internal/rules/requireatomicupdates"
	"github.com/jetlint/jetlint/internal/rules/requireawait"
	"github.com/jetlint/jetlint/internal/rules/restrictplusoperands"
	"github.com/jetlint/jetlint/internal/rules/restricttemplateexpressions"
	"github.com/jetlint/jetlint/internal/rules/returnawait"
	"github.com/jetlint/jetlint/internal/rules/strictbooleanexpressions"
	"github.com/jetlint/jetlint/internal/rules/strictvoidreturn"
	"github.com/jetlint/jetlint/internal/rules/switchexhaustivenesscheck"
	"github.com/jetlint/jetlint/internal/rules/unboundmethod"
	"github.com/jetlint/jetlint/internal/rules/useawait"
	"github.com/jetlint/jetlint/internal/rules/useerrormessage"
	"github.com/jetlint/jetlint/internal/rules/useexhaustivedependencies"
	"github.com/jetlint/jetlint/internal/rules/usehookattoplevel"
	"github.com/jetlint/jetlint/internal/rules/useimagesize"
	"github.com/jetlint/jetlint/internal/rules/useimportextensions"
	"github.com/jetlint/jetlint/internal/rules/useisnan"
	"github.com/jetlint/jetlint/internal/rules/useiterablecallbackreturn"
	"github.com/jetlint/jetlint/internal/rules/usejsonimportattributes"
	"github.com/jetlint/jetlint/internal/rules/usejsxkeyiniterable"
	"github.com/jetlint/jetlint/internal/rules/usenumbertofixeddigitsargument"
	"github.com/jetlint/jetlint/internal/rules/useparseintradix"
	"github.com/jetlint/jetlint/internal/rules/useqwikclasslist"
	"github.com/jetlint/jetlint/internal/rules/useqwikmethodusage"
	"github.com/jetlint/jetlint/internal/rules/useqwikvalidlexicalscope"
	"github.com/jetlint/jetlint/internal/rules/useselfclosingelements"
	"github.com/jetlint/jetlint/internal/rules/usesinglejsdocasterisk"
	"github.com/jetlint/jetlint/internal/rules/usesingvardeclarator"
	"github.com/jetlint/jetlint/internal/rules/usestaticresponsemethods"
	"github.com/jetlint/jetlint/internal/rules/usestrictmode"
	"github.com/jetlint/jetlint/internal/rules/useuniqueelementids"
	"github.com/jetlint/jetlint/internal/rules/useunknownincatchcallbackvariable"
	"github.com/jetlint/jetlint/internal/rules/usewhile"
	"github.com/jetlint/jetlint/internal/rules/useyield"
	"github.com/jetlint/jetlint/internal/rules/validtypeof"
	// a11y rules — JSX-only, no type checker required.
	"github.com/jetlint/jetlint/internal/rules/noaccesskey"
	"github.com/jetlint/jetlint/internal/rules/noariahiddenonfocusable"
	"github.com/jetlint/jetlint/internal/rules/noariaunsupportedelements"
	"github.com/jetlint/jetlint/internal/rules/noautofocus"
	"github.com/jetlint/jetlint/internal/rules/nodistractingelements"
	"github.com/jetlint/jetlint/internal/rules/noheaderscope"
	"github.com/jetlint/jetlint/internal/rules/nointeractiveelementtononinteractiverole"
	"github.com/jetlint/jetlint/internal/rules/nolabelwithoutcontrol"
	"github.com/jetlint/jetlint/internal/rules/nononinteractiveelementinteractions"
	"github.com/jetlint/jetlint/internal/rules/nononinteractiveelementtointeractiverole"
	"github.com/jetlint/jetlint/internal/rules/nononinteractivetabindex"
	"github.com/jetlint/jetlint/internal/rules/nopositivetabindex"
	"github.com/jetlint/jetlint/internal/rules/noredundantalt"
	"github.com/jetlint/jetlint/internal/rules/noredundantroles"
	"github.com/jetlint/jetlint/internal/rules/nostaticelementinteractions"
	"github.com/jetlint/jetlint/internal/rules/nosuspicioussemicoloninjsx"
	"github.com/jetlint/jetlint/internal/rules/nosvgwithouttitle"
	"github.com/jetlint/jetlint/internal/rules/usealttext"
	"github.com/jetlint/jetlint/internal/rules/useanchorcontent"
	"github.com/jetlint/jetlint/internal/rules/usearia"
	"github.com/jetlint/jetlint/internal/rules/useariapropsforrole"
	"github.com/jetlint/jetlint/internal/rules/useariapropssupportedbyrole"
	"github.com/jetlint/jetlint/internal/rules/usebuttontype"
	"github.com/jetlint/jetlint/internal/rules/usefocusableinteractive"
	"github.com/jetlint/jetlint/internal/rules/usegooglefontdisplay"
	"github.com/jetlint/jetlint/internal/rules/useheadingcontent"
	"github.com/jetlint/jetlint/internal/rules/usehtmllang"
	"github.com/jetlint/jetlint/internal/rules/useiframetitle"
	"github.com/jetlint/jetlint/internal/rules/usekeywithclickevents"
	"github.com/jetlint/jetlint/internal/rules/usekeywithmouseevents"
	"github.com/jetlint/jetlint/internal/rules/usemediacaption"
	"github.com/jetlint/jetlint/internal/rules/usesemanticelements"
	"github.com/jetlint/jetlint/internal/rules/usevalidanchor"
	"github.com/jetlint/jetlint/internal/rules/usevalidariaprops"
	"github.com/jetlint/jetlint/internal/rules/usevalidariarole"
	"github.com/jetlint/jetlint/internal/rules/usevalidariavalues"
	"github.com/jetlint/jetlint/internal/rules/usevalidautocomplete"
	"github.com/jetlint/jetlint/internal/rules/usevalidlang"
	"github.com/jetlint/jetlint/internal/toolerr"
	"github.com/jetlint/jetlint/internal/transport"
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// Version is the linter's reported version. Build pipelines can override
// the constant via -ldflags "-X 'github.com/jetlint/jetlint/internal/cli.Version=...'".
var Version = "0.0.0-dev"

const usage = `jetlint - fast, type-aware TypeScript linter

Usage:
    jetlint [flags] [files...]
    jetlint --project <tsconfig.json>

Flags:
    --version          Print the linter version and exit.
    --help             Print this help text and exit.
    --project <path>   Path to a tsconfig.json (or a directory containing
                       one). Required when no positional target is given;
                       positional targets win for tsconfig discovery when
                       both are provided.
    --format <name>    Output format. One of: human (default), json,
                       sarif (GitHub Code Scanning, Azure DevOps),
                       github (GitHub Actions inline PR annotations),
                       junit (CI dashboard XML),
                       rdjson (reviewdog inline PR comments).
    --max-diagnostics <n>
                       Cap on rendered diagnostics for the human format.
                       0 disables truncation. Overrides .jetlintrc.json's
                       maxDiagnostics value. Default: 20 (matches biome).
    --only <rule-id>   Restrict execution to the named rule. Repeatable
                       to allow a small set: --only no-floating-promises
                       --only no-base-to-string. Useful for head-to-head
                       comparisons against other linters.

Exit codes:
    0    No diagnostics produced.
    1    Lint diagnostics produced.
    2    Tooling failure.
`

// daemonIdleDefault is the daemon's no-request shutdown window when it is
// launched with --daemon. The plan calls out 10 minutes as the default.
const daemonIdleDefault = 10 * time.Minute

// Run parses args and executes the requested action. Returns the process
// exit code. stdout receives successful output (version, usage on --help,
// diagnostics); stderr receives error output (parse failures, tooling
// errors).
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("jetlint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	versionFlag := fs.Bool("version", false, "print version and exit")
	helpFlag := fs.Bool("help", false, "print usage and exit")
	formatFlag := fs.String("format", "human", "output format (human, json)")
	filesFromFlag := fs.String("files-from", "", "read newline-separated target paths from this file (use - for stdin)")
	// -1 is the sentinel for "not supplied"; the resolved config's
	// value (or DefaultMaxDiagnostics when no config) wins in that
	// case. 0 means "render every diagnostic".
	maxDiagnosticsFlag := fs.Int("max-diagnostics", -1, "cap on rendered diagnostics for the human format (0 = unlimited; overrides config)")
	var onlyRules stringSliceFlag
	fs.Var(&onlyRules, "only", "restrict execution to the named rule (repeatable: --only no-floating-promises --only no-base-to-string)")
	projectFlag := fs.String("project", "", "tsconfig path (or directory containing one) — required when no positional target is provided")
	daemonFlag := fs.String("daemon", "", "internal: run as the per-project daemon listening on the given socket")

	if err := fs.Parse(args); err != nil {
		// flag has already written the error and usage to stderr.
		return 2
	}

	switch {
	case *versionFlag:
		fmt.Fprintln(stdout, Version)
		return 0
	case *helpFlag:
		fmt.Fprint(stdout, usage)
		return 0
	case *daemonFlag != "":
		return runDaemon(*daemonFlag, stderr)
	}

	formatter, err := format.Lookup(*formatFlag)
	if err != nil {
		// Unknown format: report via the chosen format itself is impossible
		// because we don't have a formatter; fall back to human-formatted
		// stderr regardless and exit 2.
		emitToolError(stderr, "human",
			toolerr.WithPath(toolerr.CodeFormatUnknown, err.Error(), ""))
		return 2
	}

	targets := fs.Args()
	if *filesFromFlag != "" {
		extra, err := readFileList(*filesFromFlag)
		if err != nil {
			emitToolError(stderr, formatter.Name(),
				toolerr.WithPath(toolerr.CodeInternal, err.Error(), *filesFromFlag))
			return 2
		}
		targets = append(targets, extra...)
	}

	return runLint(targets, *projectFlag, stdout, stderr, formatter, *maxDiagnosticsFlag, onlyRules)
}

// readFileList returns the newline-separated target paths from path. The
// special path "-" reads from os.Stdin. Blank lines are ignored.
func readFileList(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// runDaemon is the entry point for the spawned daemon process. It writes
// lifecycle events to stderr; the parent CLI captures stderr into the
// per-project log file via SpawnConfig.LogPath.
func runDaemon(socketPath string, stderr io.Writer) int {
	srv, err := daemon.NewServer(socketPath, daemonIdleDefault)
	if err != nil {
		fmt.Fprintf(stderr, "jetlint daemon: start failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "jetlint daemon: started on %s, idle timeout %s\n",
		socketPath, daemonIdleDefault)
	if err := srv.Run(context.Background()); err != nil {
		fmt.Fprintf(stderr, "jetlint daemon: run failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "jetlint daemon: shut down cleanly\n")
	return 0
}

// runLint is the entry point for a normal CLI invocation. For v0.1 it
// proves the daemon round-trip end-to-end and renders the (currently
// always empty) diagnostic set via the chosen formatter.
// stringSliceFlag is a flag.Value implementation for --only and any
// future repeated string flag. Each occurrence on the command line
// appends to the slice; the value method returns a quoted joined
// string so flag's default printout stays readable.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func runLint(targets []string, projectFlag string, stdout, stderr io.Writer, formatter format.Formatter, maxDiagnosticsFlag int, onlyRules []string) int {
	// Pick the tsconfig discovery seed: an explicit --project wins; the
	// first positional target wins next. Without either, the user gets a
	// "no targets" error pointing at the flag and --help so first-time
	// users aren't left guessing (jetlint#621).
	seed := projectFlag
	if seed == "" && len(targets) > 0 {
		seed = targets[0]
	}
	if seed == "" {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeNoTargets,
				"no targets provided; pass a file or directory, or use --project <tsconfig.json> (run with --help for usage)"))
		return 2
	}

	tsconfig, err := project.FindNearestTsconfig(seed)
	if err != nil {
		code := toolerr.CodeInternal
		if project.IsNotFound(err) {
			code = toolerr.CodeTsconfigMissing
		}
		emitToolError(stderr, formatter.Name(),
			toolerr.WithPath(code, err.Error(), seed))
		return 2
	}

	// Resolve the lint configuration cascade. The first positional
	// target's directory is the user's intent when given; otherwise fall
	// back to the resolved tsconfig's directory, which is the project
	// root for an explicit --project invocation. Failures here are
	// user-facing tooling errors (bad JSON, unknown rule); they preempt
	// daemon work so the user sees the problem immediately.
	cascadeStart := filepath.Dir(tsconfig)
	if len(targets) > 0 {
		cascadeStart = filepath.Dir(targets[0])
	}
	resolved, err := config.ResolveCascade(cascadeStart)
	if err != nil {
		var te *toolerr.Error
		if errors.As(err, &te) {
			emitToolError(stderr, formatter.Name(), te)
		} else {
			emitToolError(stderr, formatter.Name(),
				toolerr.New(toolerr.CodeInternal, err.Error()))
		}
		return 2
	}

	socket, err := transport.DaemonSocketPath(tsconfig)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}

	logPath, err := transport.LogPath(tsconfig)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}
	if err := daemon.EnsureRunning(context.Background(), daemon.SpawnConfig{
		SocketPath: socket,
		LogPath:    logPath,
		Args:       []string{"--daemon", socket},
	}); err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeDaemonUnavailable, err.Error()))
		return 2
	}

	// Health-probe the daemon. On a mid-request connection drop we
	// retry exactly once after re-spawning a fresh daemon, per the
	// plan's failure-handling contract. A second failure exits 2.
	resp, err := daemon.Ping(socket, time.Second)
	if err != nil {
		if respawnErr := daemon.EnsureRunning(context.Background(), daemon.SpawnConfig{
			SocketPath: socket,
			LogPath:    logPath,
			Args:       []string{"--daemon", socket},
		}); respawnErr == nil {
			resp, err = daemon.Ping(socket, time.Second)
		}
	}
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeDaemonUnavailable, err.Error()))
		return 2
	}
	if resp.Error != "" {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, resp.Error))
		return 2
	}

	// Load the program and run the rule engine in-process. v0.1 keeps
	// the lint compute on the CLI side; the daemon's warm-path role is
	// to amortise startup once future revisions move program loading
	// into the daemon and exchange diagnostics over the socket.
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.WithPath(toolerr.CodeProgramBuildFailed, err.Error(), tsconfig))
		return 2
	}
	defer prog.Close()

	// For each explicit target, verify it is part of the discovered
	// program. Files outside the program get a per-target structured
	// warning (in JSON mode) and are skipped from the lint scope.
	// Directory targets are skipped here: they aren't single source
	// files, and the lint expands to the program's files regardless,
	// so the per-target check would emit a misleading "not part of
	// the program" error on a run that actually succeeded (jetlint#621).
	for _, target := range targets {
		abs, absErr := filepath.Abs(target)
		if absErr != nil {
			continue
		}
		if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
			continue
		}
		if prog.SourceFileByPath(abs) == nil {
			emitToolError(stderr, formatter.Name(),
				toolerr.WithPath(toolerr.CodeInternal,
					"target file is not part of the discovered TypeScript program; ensure the file is included by tsconfig.json's include/files",
					abs))
		}
	}

	// Apply --only filtering: validate every named rule against the
	// registry, then restrict the resolved severities map to just
	// those rules. Unknown rules fail fast with a structured error.
	if len(onlyRules) > 0 {
		for _, ruleID := range onlyRules {
			if !rules.IsKnown(ruleID) {
				emitToolError(stderr, formatter.Name(),
					toolerr.New(toolerr.CodeConfigUnknownRule,
						fmt.Sprintf("unknown rule %q passed via --only (known rules: %v)", ruleID, rules.MVPRuleIDs)))
				return 2
			}
		}
		filtered := make(map[string]wrapperlint.Severity, len(onlyRules))
		for _, ruleID := range onlyRules {
			if sev, ok := resolved.Rules[ruleID]; ok {
				filtered[ruleID] = sev
			} else {
				// Rule was disabled in config but explicitly requested
				// via --only; honor the override at error severity so
				// the user always sees the head-to-head comparison they
				// asked for.
				filtered[ruleID] = wrapperlint.SeverityError
			}
		}
		resolved.Rules = filtered
	}

	rulesList, ruleErr := buildRules(resolved.RuleOptions)
	if ruleErr != nil {
		emitToolError(stderr, formatter.Name(), ruleErr)
		return 2
	}
	eng := engine.New(rulesList, resolved.Rules)
	lintStart := time.Now()
	diagnostics := eng.Lint(prog)
	lintDuration := time.Since(lintStart)
	filesChecked := len(prog.SourceFiles())

	// Drop diagnostics whose source file matches a configured
	// ignorePatterns glob. The file stays in the TypeScript program so
	// its type information is still available to importers; only the
	// diagnostic emission is suppressed.
	diagnostics = filterIgnoredDiagnostics(diagnostics, resolved.IgnorePatterns)

	// Degraded-mode signal: when the program itself has type errors,
	// every type-aware diagnostic built on it is suspect. Surface a
	// single program-scope tool warning so AI agents can detect the
	// degraded state and decide how to weight the rest of the output.
	if prog.HasTypeErrors() {
		diagnostics = append([]wrapperlint.Diagnostic{{
			Range:    wrapperlint.SourceRange{File: tsconfig, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1},
			RuleID:   "jetlint/program-has-type-errors",
			Severity: wrapperlint.SeverityWarning,
			Message:  "the TypeScript program has type errors; lint diagnostics may be unreliable until those are resolved",
		}}, diagnostics...)
	}

	// The human formatter benefits from execution stats (files checked,
	// duration, max-diagnostics cap) in its summary block. Other
	// formatters don't carry that state, so we only enrich when the
	// active formatter is Human.
	maxDiagnostics := resolved.MaxDiagnostics
	if maxDiagnosticsFlag >= 0 {
		maxDiagnostics = maxDiagnosticsFlag
	}
	if _, ok := formatter.(format.Human); ok {
		formatter = format.Human{
			FilesChecked:   filesChecked,
			Duration:       lintDuration,
			MaxDiagnostics: maxDiagnostics,
		}
	}

	if err := formatter.Format(stdout, diagnostics); err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}
	if hasError(diagnostics) {
		return 1
	}
	return 0
}

// buildRules constructs every shipped rule, applying any per-rule
// options the resolved config supplied. Options for rules that don't
// accept any are rejected with a structured config error so typos
// surface at startup. The engine filters by severity, so rules
// disabled at runtime have zero overhead per node.
func buildRules(ruleOptions map[string]json.RawMessage) ([]engine.Rule, *toolerr.Error) {
	nfpOpts, err := nofloatingpromises.OptionsFromJSON(ruleOptions["no-floating-promises"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	nbtsOpts, err := nobasetotostring.OptionsFromJSON(ruleOptions["no-base-to-string"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	nmpOpts, err := nomisusedpromises.OptionsFromJSON(ruleOptions["no-misused-promises"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	acrOpts, err := arraycallbackreturn.OptionsFromJSON(ruleOptions["array-callback-return"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	uicrOpts, err := useiterablecallbackreturn.OptionsFromJSON(ruleOptions["use-iterable-callback-return"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	nicOpts, err := noimportcycles.OptionsFromJSON(ruleOptions["no-import-cycles"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	rejectIfOptionsPresent := func(ruleID string) *toolerr.Error {
		if len(ruleOptions[ruleID]) == 0 {
			return nil
		}
		return toolerr.New(toolerr.CodeConfigInvalid,
			fmt.Sprintf("rule %q does not accept options yet", ruleID))
	}
	// Rules without options support: reject any user-supplied options
	// at config-load time so typos are visible.
	rulesWithOptions := map[string]bool{
		"array-callback-return":        true,
		"use-iterable-callback-return": true,
	}
	for _, ruleID := range append([]string{
		"strict-boolean-expressions",
		"no-unsafe-assignment",
	}, rules.AdditionalTypeAwareRuleIDs...) {
		if rulesWithOptions[ruleID] {
			continue
		}
		if e := rejectIfOptionsPresent(ruleID); e != nil {
			return nil, e
		}
	}
	return []engine.Rule{
		// MVP rules with full options support.
		nofloatingpromises.NewWithOptions(nfpOpts),
		nomisleadingcharacterclass.New(),
		nomisusedpromises.NewWithOptions(nmpOpts),
		strictbooleanexpressions.New(),
		nounsafeassignment.New(),
		nobasetotostring.NewWithOptions(nbtsOpts),
		// Additional type-aware rules — default-off, opt-in via config.
		arraycallbackreturn.NewWithOptions(acrOpts),
		awaitthenable.New(),
		consistentreturn.New(),
		consistenttypeexports.New(),
		constructorsuper.New(),
		defaultcaselast.New(),
		dotnotation.New(),
		fordirection.New(),
		getterreturn.New(),
		guardforin.New(),
		namingconvention.New(),
		noadjacentspacesinregex.New(),
		noalert.New(),
		noapproximativenumericconstant.New(),
		noarguments.New(),
		noarrayindexkey.New(),
		noarraydelete.New(),
		noassigninexpressions.New(),
		noasyncpromiseexecutor.New(),
		noawaitinloop.New(),
		nobitwiseoperators.New(),
		nocatchassign.New(),
		nocommenttext.New(),
		nochildrenprop.New(),
		noclassassign.New(),
		nocommaoperator.New(),
		nocomparenegzero.New(),
		nocondassign.New(),
		noconfusinglabels.New(),
		noconfusingvoidexpression.New(),
		noconfusingvoidtype.New(),
		noconsole.New(),
		noconstantbinaryexpression.New(),
		noconstantcondition.New(),
		noconstantmathminmaxclamp.New(),
		noconstassign.New(),
		noconstenum.New(),
		noconstructorreturn.New(),
		nocontrolregex.New(),
		nodebugger.New(),
		nodeprecated.New(),
		nodeprecatedimports.New(),
		nodocumentcookie.New(),
		nodocumentimportinpage.New(),
		nodoubleequals.New(),
		nodupeargs.New(),
		nodupeclassmembers.New(),
		nodupeelseif.New(),
		noempty.New(),
		noemptycharacterclass.New(),
		noemptyinterface.New(),
		noemptypattern.New(),
		noemptysource.New(),
		noemptytypeparameters.New(),
		noevolvingtypes.New(),
		noexassign.New(),
		noexplicitany.New(),
		noexportsintest.New(),
		noextrabooleancast.New(),
		noextranonnullassertion.New(),
		nofallthrough.New(),
		noflatmapidentity.New(),
		nofocusedtests.New(),
		noforeach.New(),
		nofuncassign.New(),
		nofunctionassign.New(),
		noglobalassign.New(),
		noglobaldirnamefilename.New(),
		noglobalisfinite.New(),
		noglobalisnan.New(),
		noheadimportindocument.New(),
		noskippedtests.New(),
		nosparsearrays.New(),
		nostaticonlyclass.New(),
		nostringcasemismatch.New(),
		nosuperwithoutextends.New(),
		noswitchdeclarations.New(),
		noprecisionloss.New(),
		noprivateimports.New(),
		noprocessglobal.New(),
		nopromiseexecutorreturn.New(),
		noqwikusevisibletask.New(),
		norenderreturnvalue.New(),
		norestrictedelements.New(),
		noprototypebuiltins.New(),
		noreactforwardref.New(),
		noreactpropassignments.New(),
		noreactspecificprops.New(),
		notemplatecurlyinstring.New(),
		nothenproperty.New(),
		nothisbeforesuper.New(),
		notsignore.New(),
		notypeonlyimportattributes.New(),
		nounassignedvariables.New(),
		noredeclare.New(),
		noimportcycles.NewWithOptions(nicOpts),
		noundeclareddependencies.New(),
		novuedataobjectdeclaration.New(),
		novueduplicatekeys.New(),
		novuereservedkeys.New(),
		novuereservedprops.New(),
		novuesetuppropsreactivityloss.New(),
		nowith.New(),
		noinstanceofarray.New(),
		nomisusednew.New(),
		adjacentoverloadsignatures.New(),
		prefernamespacekeyword.New(),
		useiterablecallbackreturn.NewWithOptions(uicrOpts),
		noundef.New(),
		nodupekeys.New(),
		noduplicatecase.New(),
		noduplicateimports.New(),
		noduplicatejsxprops.New(),
		noduplicateprivateclassmembers.New(),
		noduplicatetesthooks.New(),
		noduplicatetypeconstituents.New(),
		noforinarray.New(),
		noimpliedeval.New(),
		noimplicitanylet.New(),
		noimportassign.New(),
		noinitializerwithdefinite.New(),
		noinnerdeclarations.New(),
		noinvalidbuiltininstantiation.New(),
		noinvalidregexp.New(),
		noirregularwhitespace.New(),
		nolabelvar.New(),
		nolossofprecision.New(),
		nomeaninglessvoidoperator.New(),
		nomisplacedassertion.New(),
		nomisrefactoredshorthandassign.New(),
		nomisusedspread.New(),
		nomixedenums.New(),
		nonestedcomponentdefinitions.New(),
		nonewnativenonconstructor.New(),
		nonextasyncclientcomponent.New(),
		nonodejsmodules.New(),
		nononnullassertedoptionalchain.New(),
		nononoctaldecimalescape.New(),
		noobjcalls.New(),
		nooctalescape.New(),
		nonnullabletypeassertionstyle.New(),
		noredundanttypeconstituents.New(),
		noredundantusestrict.New(),
		noselfassign.New(),
		noselfcompare.New(),
		nosoliddestructuredprops.New(),
		nosetterreturn.New(),
		noshadowrestrictednames.New(),
		nounexpectedmultiline.New(),
		nounreachable.New(),
		nounreachableloop.New(),
		nounreachablesuper.New(),
		nounresolvedimports.New(),
		nounnecessarybooleanliteralcompare.New(),
		nounnecessarycondition.New(),
		nounnecessaryqualifier.New(),
		nounnecessarytemplateexpression.New(),
		nounnecessarytypearguments.New(),
		nounnecessarytypeassertion.New(),
		nounnecessarytypeconversion.New(),
		nounnecessarytypeparameters.New(),
		nounsafeargument.New(),
		nounsafecall.New(),
		nounsafedeclarationmerging.New(),
		nounsafeenumcomparison.New(),
		nounsafememberaccess.New(),
		nounsafenegation.New(),
		nounsafeoptionalchaining.New(),
		nounsafefinally.New(),
		nounsafereturn.New(),
		nounsafetypeassertion.New(),
		nounsafeunaryminus.New(),
		nounmodifiedloopcondition.New(),
		nounusedexpressions.New(),
		nounusedfunctionparameters.New(),
		nounusedimports.New(),
		nounusedlabels.New(),
		nounusedprivateclassmembers.New(),
		nounusedvars.New(),
		nousebeforedefine.New(),
		novar.New(),
		novoidelementswithchildren.New(),
		novoidtypereturn.New(),
		nouselessbackreference.New(),
		nouselessbackreference.NewBiome(),
		nouselesscatch.New(),
		nouselesscatchbinding.New(),
		nouselesscontinue.New(),
		nouselessdefaultassignment.New(),
		nouselessemptyexport.New(),
		nouselessescapeinstring.New(),
		nouselesslabel.New(),
		nouselessrename.New(),
		nouselessstringconcat.New(),
		nouselessstringraw.New(),
		nouselessswitchcase.New(),
		nouselessternary.New(),
		nouselessundefinedinitialization.New(),
		onlythrowerror.New(),
		preferdestructuring.New(),
		preferfind.New(),
		preferincludes.New(),
		prefernullishcoalescing.New(),
		preferoptionalchain.New(),
		preferpromiserejecterrors.New(),
		preferreadonly.New(),
		preferreadonlyparametertypes.New(),
		preferreducetypeparameter.New(),
		preferregexpexec.New(),
		preferreturnthistype.New(),
		preferstringstartsendswith.New(),
		promisefunctionasync.New(),
		relatedgettersetterpairs.New(),
		requirearraysortcompare.New(),
		requireatomicupdates.New(),
		requireawait.New(),
		restrictplusoperands.New(),
		restricttemplateexpressions.New(),
		returnawait.New(),
		strictvoidreturn.New(),
		switchexhaustivenesscheck.New(),
		unboundmethod.New(),
		useawait.New(),
		useerrormessage.New(),
		useexhaustivedependencies.New(),
		usehookattoplevel.New(),
		useimagesize.New(),
		useimportextensions.New(),
		useisnan.New(),
		usejsonimportattributes.New(),
		usejsxkeyiniterable.New(),
		usenumbertofixeddigitsargument.New(),
		useparseintradix.New(),
		useqwikclasslist.New(),
		useqwikmethodusage.New(),
		useqwikvalidlexicalscope.New(),
		useselfclosingelements.New(),
		usesinglejsdocasterisk.New(),
		usesingvardeclarator.New(),
		usestaticresponsemethods.New(),
		usestrictmode.New(),
		useuniqueelementids.New(),
		useunknownincatchcallbackvariable.New(),
		usewhile.New(),
		useyield.New(),
		validtypeof.New(),
		// a11y — JSX accessibility rules.
		noaccesskey.New(),
		noariahiddenonfocusable.New(),
		noariaunsupportedelements.New(),
		noautofocus.New(),
		nodistractingelements.New(),
		noheaderscope.New(),
		nointeractiveelementtononinteractiverole.New(),
		nolabelwithoutcontrol.New(),
		nononinteractiveelementinteractions.New(),
		nononinteractiveelementtointeractiverole.New(),
		nononinteractivetabindex.New(),
		nopositivetabindex.New(),
		noredundantalt.New(),
		noredundantroles.New(),
		nostaticelementinteractions.New(),
		nosuspicioussemicoloninjsx.New(),
		nosvgwithouttitle.New(),
		usealttext.New(),
		useanchorcontent.New(),
		usearia.New(),
		useariapropsforrole.New(),
		useariapropssupportedbyrole.New(),
		usebuttontype.New(),
		usefocusableinteractive.New(),
		usegooglefontdisplay.New(),
		useheadingcontent.New(),
		usehtmllang.New(),
		useiframetitle.New(),
		usekeywithclickevents.New(),
		usekeywithmouseevents.New(),
		usemediacaption.New(),
		usesemanticelements.New(),
		usevalidanchor.New(),
		usevalidariaprops.New(),
		usevalidariarole.New(),
		usevalidariavalues.New(),
		usevalidautocomplete.New(),
		usevalidlang.New(),
	}, nil
}

// hasError reports whether any diagnostic in the slice was emitted at
// error severity (the signal for exit code 1).
// filterIgnoredDiagnostics removes diagnostics whose file matches the
// resolved ignorePatterns matcher. Diagnostic.Range.File is already an
// absolute path produced by the TypeScript program.
func filterIgnoredDiagnostics(diags []wrapperlint.Diagnostic, m config.IgnoreMatcher) []wrapperlint.Diagnostic {
	out := diags[:0]
	for _, d := range diags {
		if m.Matches(d.Range.File) {
			continue
		}
		out = append(out, d)
	}
	return out
}

func hasError(d []wrapperlint.Diagnostic) bool {
	for _, x := range d {
		if x.Severity == wrapperlint.SeverityError {
			return true
		}
	}
	return false
}

// emitToolError writes a tooling failure to stderr in the appropriate
// shape for the given format. JSON mode emits a single-line JSON object;
// human mode emits a "jetlint: <message>" line.
func emitToolError(stderr io.Writer, formatName string, e *toolerr.Error) {
	if formatName == "json" {
		_ = e.WriteJSON(stderr)
		return
	}
	if e.Path != "" {
		fmt.Fprintf(stderr, "jetlint: %s: %s\n", e.Path, e.Message)
	} else {
		fmt.Fprintf(stderr, "jetlint: %s\n", e.Message)
	}
}

// Ensure errors package import is reachable from here even if all error
// inspection currently lives in helpers; this keeps a stable surface.
var _ = errors.Is
