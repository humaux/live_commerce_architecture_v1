// Loading skeleton while this list page reads the catalog (see components/Skeletons.tsx). No BFF/Go call.
import { GridSkeleton } from "../../../../components/Skeletons";

export default function Loading() {
  return <GridSkeleton />;
}
