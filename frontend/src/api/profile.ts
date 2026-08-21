import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "./client";
import { keys } from "./queryKeys";
import type { OwnProfile, ProfileUpdateInput } from "./types";

export function useOwnProfile() {
  return useQuery({
    queryKey: keys.me.profile(),
    queryFn: async () => (await api.get<OwnProfile>("/me/profile")).data,
  });
}

export function useUpdateOwnProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: ProfileUpdateInput) =>
      (await api.patch<OwnProfile>("/me/profile", input)).data,
    onSuccess: (updated) => {
      // Seed the cache with the server's answer so the change renders
      // immediately, then invalidate to catch anything a refetch would see.
      queryClient.setQueryData(keys.me.profile(), updated);
      queryClient.invalidateQueries({ queryKey: keys.me.profile() });
    },
  });
}
