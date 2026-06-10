  @Post(':id/contact')
  async createContact(@Param('id') id: string, @Body() dto: CreateContactDto) {
    const contract = await this.contracts.findOne(id);
    if (contract === null) {
      throw new NotFoundException('contract not found');
    }
    if (contract.client === null) {
      throw new HttpException('contract has no client', 400);
    }
    const existing = await this.contacts.findByEmail(dto.email);
    if (existing !== null) {
      throw new HttpException('contact already exists', 409);
    }
    try {
      const contact = await this.contacts.create(contract, dto);
      return { success: true, contact };
    } catch (e) {
      this.logger.error(e);
      throw new HttpException('failed to create contact', 500);
    }
  }

  @Put(':id/contact/:contactId')
  async updateContact(
    @Param('id') id: string,
    @Param('contactId') contactId: string,
    @Body() dto: UpdateContactDto,
  ) {
    const contact = await this.contacts.findOne(contactId);
    if (contact === null) {
      throw new NotFoundException('contact does not exist');
    }
    const conflict = await this.contacts.findByEmail(dto.email);
    if (conflict !== null && conflict.id !== contactId) {
      throw new HttpException('email conflict', 422);
    }
    try {
      return await this.contacts.update(contact, dto);
    } catch (e) {
      this.logger.error(e);
      throw new HttpException('failed to update contact', 503);
    }
  }
