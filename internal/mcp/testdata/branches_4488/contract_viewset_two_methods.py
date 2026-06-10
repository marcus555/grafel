    @action(methods=["post"], detail=True, serializer_class=ContractContactCreateSerializer)
    def create_contact(self, request, pk, *args, **kwargs):
        try:
            contract = self.get_object()
            client = contract.client
            if client is None:
                return Response(
                    {
                        "success": False,
                        "message": "Contract has no client to attach the contact to.",
                    },
                    status=status.HTTP_400_BAD_REQUEST,
                )

            serializer = ContractContactCreateSerializer(data=request.data)
            serializer.is_valid(raise_exception=True)
            data = serializer.validated_data

            upsert_flag = data.get("upsert") or str(
                request.query_params.get("upsert", "false")
            ).lower() == "true"

            email = data["email"]
            email_availability = check_contact_email_availability({"email": email})

            if email_availability["available"] is False and not upsert_flag:
                existing = User.objects.filter(email__iexact=email).first()
                return Response(
                    {
                        "error": "User with this email/username already exists.",
                        "existing_user": UserSerializer(existing).data if existing else None,
                    },
                    status=status.HTTP_409_CONFLICT,
                )

            if email_availability["available"] is False:
                user = User.objects.filter(email__iexact=email).first()
                if user is not None:
                    attach_client_to_contact(user, client)
                    attach_contract_to_contact(user, contract)
                    return Response(
                        {
                            "success": True,
                            "message": "Existing contact linked to contract",
                            "contact": user.id,
                        },
                        status=status.HTTP_200_OK,
                    )

            # Create new user, attach to client (M2M + legacy FK), then to contract
            user = create_contact_for_client(data, client)
            attach_contract_to_contact(user, contract)
            return Response(
                {
                    "success": True,
                    "message": "Contact has been created and linked to contract",
                    "contact": user.id,
                },
                status=status.HTTP_201_CREATED,
            )
        except Exception as e:
            return Response(
                {
                    "success": False,
                    "message": "There was an error while creating the contact.",
                    "errors": str(e),
                },
                status=status.HTTP_500_INTERNAL_SERVER_ERROR,
            )


    @action(methods=["patch"], detail=True, serializer_class=ContractContactUpdateSerializer)
    def update_contact(self, request, pk, *args, **kwargs):
        try:
            contract = self.get_object()
            client = contract.client

            serializer = ContractContactUpdateSerializer(data=request.data)
            serializer.is_valid(raise_exception=True)
            data = serializer.validated_data

            try:
                original_user = User.objects.get(pk=data["id"])
            except User.DoesNotExist:
                return Response(
                    {"error": "User not found"},
                    status=status.HTTP_404_NOT_FOUND,
                )

            upsert_flag = data.get("upsert") or str(
                request.query_params.get("upsert", "false")
            ).lower() == "true"

            # Detect email collisions with other users; upsert re-targets the update.
            user, conflict_user = resolve_contact_email_conflict(
                original_user, data.get("email"), upsert_flag
            )
            if conflict_user is not None:
                return Response(
                    {
                        "error": "User with this email/username already exists.",
                        "existing_user": UserSerializer(conflict_user).data,
                    },
                    status=status.HTTP_409_CONFLICT,
                )

            # Upsert special case: when the resolved user is a different existing
            # contact, move them onto this contract (and its client) and unassign
            # the original user from this contract/client since the resolved one
            # replaces them here.
            if user.pk != original_user.pk:
                UserContract.objects.filter(user=original_user, contract=contract).delete()
                if client is not None:
                    original_user.clients.remove(client)
                    if original_user.client_id == client.id:
                        original_user.client_id = None
                        original_user.save(update_fields=["client_id"])
                    attach_client_to_contact(user, client)
                attach_contract_to_contact(user, contract)

            # Partial update: only fields present in `data` are written
            update_base_contact(user, data)

            # Keep legacy FK in sync with the contract's client and ensure group/role/M2M linkage
            if client is not None and user.client_id != client.id:
                user.client_id = client.id
                user.save(update_fields=["client_id"])
            ensure_client_contact_membership(user, client)

            # Ensure the user is linked to this contract (active UserContract)
            ensure_contract_contact_membership(user, contract)

            return Response(
                {"success": True, "message": "Contact has been updated", "contact": user.id},
                status=status.HTTP_200_OK,
            )
        except Exception as e:
            return Response(
                {
                    "success": False,
                    "message": "An error occurred while updating the contact",
                    "errors": str(e),
                },
                status=status.HTTP_500_INTERNAL_SERVER_ERROR,
            )
