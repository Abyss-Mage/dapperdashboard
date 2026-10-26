"""Copy web source for a build that never touches the running app's .next directory."""
from pathlib import Path
import shutil
root=Path(__file__).resolve().parent.parent
source=root/'dapper-platform/apps/web'
target=root/'.cache/website-validation/dapper-platform/apps/web'
shutil.copytree(source,target,dirs_exist_ok=True,ignore=shutil.ignore_patterns('node_modules','.next','.git','*.tsbuildinfo','.env*'))
generated=root/'.cache/website-validation/main website'
generated.mkdir(parents=True,exist_ok=True)
shutil.copy2(root/'main website/pages.generated.ts',generated/'pages.generated.ts')
print('Prepared isolated web source. Add a node_modules junction, then run next build --webpack there.')
